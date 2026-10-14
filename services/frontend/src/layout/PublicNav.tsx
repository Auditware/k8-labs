import { Link, useLocation } from "react-router-dom";
import { Trophy } from "lucide-react";

import { useAuth } from "../context/AuthContext";
import { Avatar } from "../components/ui/Avatar";
import { ServerIssueBadge } from "../components/ServerIssueBadge";

// The top bar shared by the public, standalone pages (landing hero, leaderboard,
// login). Extracted so the bar is byte-identical across them - it must not shift
// appearance as you move between these pages. The landing keeps its own richer
// copy, but the serverDown badge and mount skeleton behave identically here.
//
// When the backend is unreachable, a centered "server issues" badge is overlaid
// in the middle of the bar (absolute, so it centers on the bar itself and is
// unaffected by the widths of the logo and right-side controls). It's the only
// place this indicator lives on public pages - see ServerIssueBadge.
//
// It's a fixed, full-width h-20 bar; the page below it owns its own top padding
// to clear it.
export function PublicNav() {
  const { isAuthenticated, isLoading, serverDown, user, avatarVersion } = useAuth();
  const { pathname } = useLocation();

  return (
    <nav className="fixed inset-x-0 top-0 z-50 border-b border-hairline/40 bg-slate-950/70 backdrop-blur">
      <div className="relative flex h-20 items-center justify-between px-4">
        {serverDown && (
          <div className="pointer-events-none absolute inset-0 flex items-center justify-center">
            <div className="pointer-events-auto">
              <ServerIssueBadge />
            </div>
          </div>
        )}
        <Link
          to="/"
          className="flex items-center gap-3 rounded-lg text-lg font-semibold text-slate-100 transition-colors hover:text-white"
          aria-label="K8 Labs home"
        >
          <img src="/logo.svg" alt="" className="h-12 w-auto" />
          K8 Labs
        </Link>
        <div className="flex items-center gap-3">
          <Link
            to="/leaderboard"
            aria-current={pathname === "/leaderboard" ? "page" : undefined}
            className="inline-flex h-9 items-center gap-1.5 rounded-lg px-3 text-13 text-slate-400 transition-colors hover:bg-white/5 hover:text-slate-100 focus:outline-none focus-visible:ring-2 focus-visible:ring-accent"
          >
            <Trophy size={15} className="text-slate-500" aria-hidden />
            Leaderboard
          </Link>
          {isLoading ? (
            // Hold the corner with a pulsing dot until the auth probe resolves, so
            // a signed-in member refreshing doesn't see the "Sign in" button flash
            // before their avatar lands.
            <div className="h-9 w-9 animate-pulse rounded-full bg-slate-800" aria-hidden />
          ) : isAuthenticated && user ? (
            <Link
              to="/labs"
              aria-label="Go to your labs"
              className="flex h-9 w-9 items-center justify-center rounded-full ring-1 ring-hairline transition hover:ring-hairline-strong focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
            >
              <Avatar user={user} size={36} version={avatarVersion} />
            </Link>
          ) : (
            <Link
              to="/login"
              className="inline-flex h-9 items-center rounded-lg bg-accent/10 px-3.5 text-13 font-medium text-accent ring-1 ring-inset ring-accent/25 transition-colors hover:bg-accent/15 hover:ring-accent/40 focus:outline-none focus-visible:ring-2 focus-visible:ring-accent"
            >
              Sign in
            </Link>
          )}
        </div>
      </div>
    </nav>
  );
}
