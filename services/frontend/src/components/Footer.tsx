import { Link } from "react-router-dom";
import { Send } from "lucide-react";
import AccessibilityWidget from "./AccessibilityWidget";

// Ported from our shared component set. Full-width (to match the landing's edge-aligned
// nav) rather than the original's centred max-w-7xl. Theme classes remapped to k8 labs'
// slate/accent/hairline tokens. Social links are forefy's own; Terms/Privacy
// point at the stub pages wired in App.tsx.
export default function Footer() {
  const linkClass =
    "rounded text-slate-500 underline underline-offset-2 transition-colors hover:text-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent";
  return (
    <footer
      className="border-t border-hairline/50 px-6 py-4 backdrop-blur-sm md:px-8"
      role="contentinfo"
      aria-label="Site footer"
    >
      <div className="flex flex-col items-center justify-between gap-4 md:flex-row">
        <p className="text-sm text-slate-500">
          © 2026 forefy | By using this site you agree to our{" "}
          <Link to="/terms" className={linkClass} aria-label="Read our Terms of Service">
            Terms of Service
          </Link>{" "}
          and{" "}
          <Link to="/privacy" className={linkClass} aria-label="Read our Privacy Policy">
            Privacy Policy
          </Link>
          .
        </p>
        <nav aria-label="Social links" className="flex items-center gap-6">
          <a
            href="https://x.com/forefy"
            target="_blank"
            rel="noopener noreferrer"
            className="rounded text-slate-500 transition-colors hover:text-white/70 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
            aria-label="Follow forefy on X (opens in new tab)"
          >
            <svg
              className="h-4 w-4"
              viewBox="0 0 24 24"
              fill="currentColor"
              aria-hidden="true"
              focusable="false"
            >
              <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-4.714-6.231-5.401 6.231H2.744l7.73-8.835L1.254 2.25H8.08l4.253 5.622zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
            </svg>
          </a>
          <a
            href="https://t.me/forefy_t"
            target="_blank"
            rel="noopener noreferrer"
            className="rounded text-slate-500 transition-colors hover:text-[#2AABEE] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
            aria-label="Contact forefy on Telegram (opens in new tab)"
          >
            <Send className="h-4 w-4" aria-hidden="true" />
          </a>
          <AccessibilityWidget />
        </nav>
      </div>
    </footer>
  );
}
