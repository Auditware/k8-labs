import { useEffect } from "react";

// Runtime per-route <head> management, no dependency (react-helmet et al.) -
// the app already ships an empty CSR shell, so we keep the SEO surface tiny and
// framework-free. index.html carries sensible site-wide defaults; this hook
// overrides them per route on the client, and serve.ts rewrites the same tags
// server-side for known public routes so non-JS crawlers/unfurlers agree with
// what the SPA would render. Keep the two in sync (services/frontend/serve.ts
// PUBLIC_META) when you change a public route's copy.

export const SITE_URL = "https://k8labs.example.com";
export const SITE_NAME = "K8 Labs";
export const DEFAULT_OG_IMAGE = `${SITE_URL}/media/ebpf-lab-demo.poster.jpg`;

export interface SeoOptions {
  /** Full <title>. If omitted, the site default title is kept. */
  title?: string;
  description?: string;
  /** Absolute path for the canonical/og:url, e.g. "/leaderboard". */
  path?: string;
  /** Absolute image URL for og/twitter. Defaults to the site OG image. */
  image?: string;
  /** true => noindex,nofollow (auth-gated / utility routes). */
  noindex?: boolean;
  /** og:type - "website" (default) or "profile" for operator pages. */
  type?: "website" | "profile" | "article";
  /** Optional JSON-LD object injected as a per-route <script>. */
  jsonLd?: Record<string, unknown>;
}

function upsertMeta(attr: "name" | "property", key: string, content: string) {
  let el = document.head.querySelector<HTMLMetaElement>(`meta[${attr}="${key}"]`);
  if (!el) {
    el = document.createElement("meta");
    el.setAttribute(attr, key);
    document.head.appendChild(el);
  }
  el.setAttribute("content", content);
}

function upsertCanonical(href: string) {
  let el = document.head.querySelector<HTMLLinkElement>('link[rel="canonical"]');
  if (!el) {
    el = document.createElement("link");
    el.setAttribute("rel", "canonical");
    document.head.appendChild(el);
  }
  el.setAttribute("href", href);
}

// Applies SEO tags for the current route. Safe to call unconditionally; pass a
// partial object and only those tags are touched (title/description always are,
// falling back to the site defaults so a previous route's copy never lingers).
export function useSeo(opts: SeoOptions): void {
  const {
    title,
    description = "Hands-on Kubernetes attack & defense labs. Drop into a real, disposable cluster from a root shell, break the target, and earn a verifiable rank when you clear each mission.",
    path,
    image = DEFAULT_OG_IMAGE,
    noindex = false,
    type = "website",
    jsonLd,
  } = opts;

  const fullTitle = title ?? `${SITE_NAME} - Hands-on Kubernetes Attack & Defense`;
  const url = path ? `${SITE_URL}${path}` : SITE_URL + "/";

  // Serialize the JSON-LD outside the effect so its identity is stable across
  // renders (object literals would otherwise refire the effect every render).
  const jsonLdStr = jsonLd ? JSON.stringify(jsonLd) : "";

  useEffect(() => {
    document.title = fullTitle;
    upsertMeta("name", "description", description);
    upsertMeta(
      "name",
      "robots",
      noindex ? "noindex, nofollow" : "index, follow, max-image-preview:large",
    );
    upsertCanonical(url);

    upsertMeta("property", "og:title", fullTitle);
    upsertMeta("property", "og:description", description);
    upsertMeta("property", "og:url", url);
    upsertMeta("property", "og:image", image);
    upsertMeta("property", "og:type", type);

    upsertMeta("name", "twitter:title", fullTitle);
    upsertMeta("name", "twitter:description", description);
    upsertMeta("name", "twitter:image", image);

    // Per-route JSON-LD lives in its own tagged <script> so we can replace it
    // on navigation without disturbing the static graph in index.html.
    const existing = document.head.querySelector('script[data-seo-route="1"]');
    if (jsonLdStr) {
      const script = existing ?? document.createElement("script");
      script.setAttribute("type", "application/ld+json");
      script.setAttribute("data-seo-route", "1");
      script.textContent = jsonLdStr;
      if (!existing) document.head.appendChild(script);
    } else if (existing) {
      existing.remove();
    }
  }, [fullTitle, description, url, image, noindex, type, jsonLdStr]);
}
