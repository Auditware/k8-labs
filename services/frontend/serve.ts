// Runtime static file server for the built SPA - used only in the final
// Docker stage. Deliberately Bun instead of adding nginx as a third
// runtime image alongside Go and Bun (see docs/architecture/tech-stack.md).
//
// Beyond plain static serving it does two SEO-relevant things a naive SPA
// fallback can't:
//   1. Real 404s for missing files that have an extension (favicon.ico,
//      *.png, *.map, ...). The old blanket "serve index.html for anything
//      unknown" turned every such miss into a 200 soft-404, which wastes
//      crawl budget and lets junk URLs get indexed.
//   2. Per-route <head> rewriting for the handful of known public routes, so
//      non-JS crawlers and social unfurlers (LinkedIn/Slack/X/iMessage - none
//      run JS) see the right title/description/canonical/og tags instead of
//      the generic shell. The client mirrors this at runtime via
//      src/lib/seo.ts (useSeo); keep the two copies in sync.
const dist = new URL("./dist", import.meta.url).pathname;
const port = Number(process.env.PORT ?? 8080);

const template = await Bun.file(`${dist}/index.html`).text();

const SITE_URL = "https://k8labs.example.com";
const DEFAULT_DESC =
  "Hands-on Kubernetes attack & defense labs. Drop into a real, disposable cluster from a root shell, break the target, and earn a verifiable rank when you clear each mission.";

interface RouteMeta {
  title: string;
  description: string;
  path: string;
}

// Keep in sync with the per-page useSeo() calls in src/. Dynamic routes
// (/u/:username, /labs/:slug) are intentionally absent: their copy depends on
// data the edge server doesn't have, so they fall through to the default
// shell and the client fills in the tags.
const PUBLIC_META: Record<string, RouteMeta> = {
  "/": {
    title: "K8 Labs - Hands-on Kubernetes Attack & Defense",
    description: DEFAULT_DESC,
    path: "/",
  },
  "/leaderboards": {
    title: "Leaderboards - K8 Labs",
    description:
      "The K8 Labs operator leaderboard: fastest roots, longest streaks, and top-percentile ranks across the Kubernetes attack & defense labs.",
    path: "/leaderboards",
  },
  "/terms": {
    title: "Terms of Service - K8 Labs",
    description: "Terms of Service for K8 Labs.",
    path: "/terms",
  },
  "/privacy": {
    title: "Privacy Policy - K8 Labs",
    description: "Privacy Policy for K8 Labs.",
    path: "/privacy",
  },
};

function esc(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

function setMeta(html: string, attr: "name" | "property", key: string, value: string): string {
  const re = new RegExp(`(<meta ${attr}="${key}" content=")[^"]*(")`);
  return html.replace(re, `$1${esc(value)}$2`);
}

// Rewrite the single-line title/description/canonical/og/twitter tags of the
// shell for a known route. Any tag not present is simply left untouched.
function renderRoute(meta: RouteMeta): string {
  const url = `${SITE_URL}${meta.path}`;
  let out = template.replace(/<title>[\s\S]*?<\/title>/, `<title>${esc(meta.title)}</title>`);
  out = out.replace(/(<link rel="canonical" href=")[^"]*(")/, `$1${url}$2`);
  out = setMeta(out, "name", "description", meta.description);
  out = setMeta(out, "property", "og:title", meta.title);
  out = setMeta(out, "property", "og:description", meta.description);
  out = setMeta(out, "property", "og:url", url);
  out = setMeta(out, "name", "twitter:title", meta.title);
  out = setMeta(out, "name", "twitter:description", meta.description);
  return out;
}

// Pre-render the known routes once at startup - they never change at runtime.
const RENDERED: Record<string, string> = {};
for (const [path, meta] of Object.entries(PUBLIC_META)) {
  RENDERED[path] = renderRoute(meta);
}

// no-cache (not no-store): the browser may keep index.html but MUST revalidate
// with the origin before using it, so a deploy that changes the hashed bundle
// reference is picked up on the very next load instead of lingering behind a
// heuristically-cached shell. Pairs with the immutable /assets/* headers below.
const htmlHeaders = { "content-type": "text/html; charset=utf-8", "cache-control": "no-cache" };
function htmlResponse(body: string, status = 200): Response {
  return new Response(body, { status, headers: htmlHeaders });
}

// A path whose last segment has a file extension (".ico", ".png", ".map"...).
// Such a path is an asset request, never a client-side route - so a miss is a
// genuine 404 rather than an SPA deep link.
function looksLikeFile(pathname: string): boolean {
  return /\.[a-z0-9]+$/i.test(pathname.slice(pathname.lastIndexOf("/")));
}

Bun.serve({
  port,
  async fetch(req) {
    const { pathname } = new URL(req.url);

    // Known public route → shell with route-specific <head>.
    const rendered = RENDERED[pathname];
    if (rendered) return htmlResponse(rendered);

    // Real static asset (JS/CSS/images, robots.txt, sitemap.xml, manifest).
    const file = Bun.file(`${dist}${pathname}`);
    if (await file.exists()) {
      const res = new Response(file);
      // Vite content-hashes everything under /assets/ (index-<hash>.js, etc), so
      // a given URL's bytes never change - safe to cache hard and skip revalidation.
      // Non-hashed files (robots.txt, sitemap.xml, favicon) keep default handling
      // so a deploy that changes them isn't pinned behind a year-long cache.
      if (pathname.startsWith("/assets/")) {
        res.headers.set("cache-control", "public, max-age=31536000, immutable");
      }
      return res;
    }

    // Missing asset → honest 404 (fixes soft-404s on favicon.ico, *.png, etc).
    if (looksLikeFile(pathname)) return new Response("Not found", { status: 404 });

    // Extensionless unknown path → a client-side route; hand back the shell.
    return htmlResponse(template);
  },
});

console.log(`frontend listening on :${port}`);
