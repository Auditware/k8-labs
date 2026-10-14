// Shown in the top bar when the auth probe has exhausted its retries and can't
// reach the backend (see AuthContext.serverDown). It replaces the "Sign in"
// button, because signing in is exactly what won't work while the server is
// down - so instead of a dead-end, the user gets a way to report it.
//
// Deliberately "calm red": a muted tint and a small steady dot, not a loud
// alert. It states the problem and hands off to a human on Telegram; the whole
// badge is the link to @forefy_t.

const TELEGRAM_URL = "https://t.me/forefy_t";

// Telegram's mark, inlined so it needs no external request (the prod CSP blocks
// third-party image hosts) and inherits the badge's text colour via currentColor.
function TelegramIcon({ className = "" }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden className={className}>
      <path d="M11.944 0A12 12 0 0 0 0 12a12 12 0 0 0 12 12 12 12 0 0 0 12-12A12 12 0 0 0 12 0a12 12 0 0 0-.056 0zm4.962 7.224c.1-.002.321.023.465.14a.506.506 0 0 1 .171.325c.016.093.036.306.02.472-.18 1.898-.962 6.502-1.36 8.627-.168.9-.499 1.201-.82 1.23-.696.065-1.225-.46-1.9-.902-1.056-.693-1.653-1.124-2.678-1.8-1.185-.78-.417-1.21.258-1.91.177-.184 3.247-2.977 3.307-3.23.007-.032.014-.15-.056-.212s-.174-.041-.249-.024c-.106.024-1.793 1.14-5.061 3.345-.48.33-.913.49-1.302.48-.428-.008-1.252-.241-1.865-.44-.752-.245-1.349-.374-1.297-.789.027-.216.325-.437.893-.663 3.498-1.524 5.83-2.529 6.998-3.014 3.332-1.386 4.025-1.627 4.476-1.635z" />
    </svg>
  );
}

export function ServerIssueBadge() {
  return (
    <a
      href={TELEGRAM_URL}
      target="_blank"
      rel="noreferrer noopener"
      title="Report a server issue to @forefy_t on Telegram"
      className="inline-flex items-center gap-2 rounded-lg border border-red-500/30 bg-red-500/10
        px-3 py-1.5 text-13 font-medium text-red-300 transition-colors
        hover:border-red-500/50 hover:bg-red-500/15 hover:text-red-200
        focus:outline-none focus-visible:ring-2 focus-visible:ring-red-500/60"
    >
      <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-red-400" aria-hidden />
      <span>Server issues, please report to us</span>
      <TelegramIcon className="h-4 w-4 shrink-0" />
    </a>
  );
}
