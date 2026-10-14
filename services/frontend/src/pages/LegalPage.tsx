import { Link } from "react-router-dom";
import { useSeo } from "../lib/seo";

// Placeholder for the footer's Terms/Privacy links (see components/Footer.tsx).
// Public route - no real legal copy yet, deliberately not fabricated.
export default function LegalPage({ title }: { title: string }) {
  const isPrivacy = title.toLowerCase().includes("privacy");
  useSeo({
    title: `${title} - K8 Labs`,
    description: `${title} for K8 Labs.`,
    path: isPrivacy ? "/privacy" : "/terms",
  });
  return (
    <div className="min-h-screen bg-slate-950 px-6 py-24 text-slate-100">
      <div className="mx-auto max-w-2xl">
        <Link to="/" className="font-mono text-13 text-accent hover:underline">
          ← back to K8 Labs
        </Link>
        <h1 className="mt-8 text-3xl font-bold tracking-tight">{title}</h1>
        <p className="mt-4 text-slate-400">
          This page is coming soon. Reach out on{" "}
          <a
            href="https://t.me/forefy_t"
            target="_blank"
            rel="noopener noreferrer"
            className="text-accent hover:underline"
          >
            Telegram
          </a>{" "}
          with any questions in the meantime.
        </p>
      </div>
    </div>
  );
}
