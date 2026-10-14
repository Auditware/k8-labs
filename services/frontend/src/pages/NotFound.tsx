import { Link } from "react-router-dom";
import { useSeo } from "../lib/seo";

// Catch-all for any path React Router doesn't recognise. The server still
// answers 200 (it can't tell a real client route from a typo without
// enumerating them), so the noindex here is what actually keeps stray/junk
// URLs out of the index.
export default function NotFound() {
  useSeo({ title: "Page not found - K8 Labs", noindex: true });
  return (
    <div className="flex min-h-screen items-center justify-center bg-slate-950 px-6 text-slate-100">
      <div className="text-center">
        <p className="font-mono text-13 uppercase tracking-wider text-slate-500">404</p>
        <h1 className="mt-3 text-2xl font-bold tracking-tight">This page doesn&rsquo;t exist</h1>
        <p className="mt-2 text-sm text-slate-400">
          The link may be broken or the page may have moved.
        </p>
        <Link
          to="/"
          className="mt-6 inline-block font-mono text-13 text-accent hover:underline"
        >
          ← back to K8 Labs
        </Link>
      </div>
    </div>
  );
}
