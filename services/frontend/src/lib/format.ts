// Shared operator-stat formatters, so the leaderboard, the landing deck, and the
// profile page all render the same numbers the same way.

// A solve time in seconds as m:ss (or h mm past an hour).
export function formatSolve(s: number): string {
  if (s < 3600) {
    const m = Math.floor(s / 60);
    const ss = s % 60;
    return `${m}:${String(ss).padStart(2, "0")}`;
  }
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  return `${h}h ${String(m).padStart(2, "0")}m`;
}

// "joined 4 months ago" from an ISO timestamp, in the coarsest honest unit.
export function joinedAgo(iso: string): string {
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "";
  const days = Math.floor((Date.now() - then) / 86_400_000);
  if (days < 1) return "joined today";
  if (days < 30) return `joined ${days} day${days === 1 ? "" : "s"} ago`;
  const months = Math.floor(days / 30);
  if (months < 12) return `joined ${months} month${months === 1 ? "" : "s"} ago`;
  const years = Math.floor(days / 365);
  return `joined ${years} year${years === 1 ? "" : "s"} ago`;
}

// Compact "time since" for an ISO timestamp - "3h ago", "2d ago", "just now".
// Used where space is tight (the admin user table), so it stays short rather
// than the prose joinedAgo produces. Empty string for a missing/invalid value.
export function timeAgo(iso?: string): string {
  if (!iso) return "";
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "";
  const secs = Math.floor((Date.now() - then) / 1000);
  if (secs < 60) return "just now";
  const mins = Math.floor(secs / 60);
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d ago`;
  const months = Math.floor(days / 30);
  if (months < 12) return `${months}mo ago`;
  return `${Math.floor(days / 365)}y ago`;
}

// Real "top percentile" from an operator's rank among all ranked operators.
// null for a board of one, where a percentile says nothing - callers drop the
// stat rather than print a meaningless "top 100%".
export function topPercentile(rank: number, total: number): string | null {
  if (total <= 1) return null;
  const pct = (rank / total) * 100;
  return `${pct < 1 ? pct.toFixed(1) : Math.round(pct)}%`;
}
