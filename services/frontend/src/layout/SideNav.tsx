import { Fragment, useState, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link, NavLink, useLocation, useNavigate } from "react-router-dom";
import {
  Check,
  ChevronDown,
  ChevronRight,
  FlaskConical,
  HardDrive,
  LayoutDashboard,
  Lock,
  LogOut,
  PanelLeftClose,
  PanelLeftOpen,
  Settings,
  Terminal,
  Users,
  X,
} from "lucide-react";
import { getModulesWithMeta } from "../lib/api";
import { LeaderboardTarget } from "./LeaderboardTarget";
import { useAuth } from "../context/AuthContext";
import { useIsNarrow } from "../lib/useIsNarrow";
import { Avatar } from "../components/ui/Avatar";

const COLLAPSED_KEY = "k8labs:sidebar-collapsed";
const TRACKS_COLLAPSED_KEY = "k8labs:tracks-collapsed";

// The catalog is grouped by tactic, not by completion. TRACKS is the display
// order and labels for those groups; the key matches labs.Module.Track (set by
// the backend's trackOf map). First Flight is the mixed onboarding sampler, then
// the tracks follow the attack lifecycle: recon, escalate, move, break out,
// persist. Labs sort by the server's Order within each group. A module whose
// track is not listed here falls into a trailing "More" group rather than
// vanishing (see the grouping in SideNav).
// Track glyph: a hand-built line icon in the sidebar's lucide style (24 viewBox,
// currentColor stroke) so it inherits the header's colour and hover state.
function TrackGlyph({ paths, size = 14 }: { paths: ReactNode; size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={2}
      strokeLinecap="round"
      strokeLinejoin="round"
      className="shrink-0"
      aria-hidden="true"
    >
      {paths}
    </svg>
  );
}

// The six tracks: display order, short label, and the chosen glyph's raw SVG
// paths. Rendered through TrackGlyph at 14px in the expanded headers and 19px in
// the collapsed rail (to match the other rail icons), hence paths not a fixed
// element.
const TRACKS: { key: string; label: string; paths: ReactNode }[] = [
  {
    key: "first-flight",
    label: "First flight",
    paths: (
      <>
        <path d="M4.5 16.5c-1.5 1.3-2 5-2 5s3.7-.5 5-2c.7-.8.7-2.1-.1-2.9a2.2 2.2 0 0 0-2.9-.1z" />
        <path d="m12 15-3-3a22 22 0 0 1 2-4A12.9 12.9 0 0 1 22 2c0 2.7-.8 7.5-6 11a22 22 0 0 1-4 2z" />
        <path d="M9 12H4s.6-3 2-4c1.6-1.1 5 0 5 0" />
        <path d="M12 15v5s3-.6 4-2c1.1-1.6 0-5 0-5" />
      </>
    ),
  },
  {
    key: "recon",
    label: "Recon",
    paths: (
      <>
        <circle cx="12" cy="12" r="10" />
        <line x1="22" y1="12" x2="18" y2="12" />
        <line x1="6" y1="12" x2="2" y2="12" />
        <line x1="12" y1="6" x2="12" y2="2" />
        <line x1="12" y1="22" x2="12" y2="18" />
      </>
    ),
  },
  {
    key: "privesc",
    label: "Privesc",
    paths: <path d="M3 21h5v-5h5v-5h5V6h3" />,
  },
  {
    key: "lateral",
    label: "Lateral movement",
    paths: (
      <>
        <rect x="2" y="8" width="7" height="8" rx="1.5" />
        <rect x="15" y="8" width="7" height="8" rx="1.5" />
        <path d="M9 12h6" />
        <path d="m13 10 2 2-2 2" />
      </>
    ),
  },
  {
    key: "escape",
    label: "Node escape",
    paths: (
      <>
        <path d="M14 4H6a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-8" />
        <path d="M14 10 21 3" />
        <path d="M15 3h6v6" />
      </>
    ),
  },
  {
    key: "persistence",
    label: "Persistence",
    paths: (
      <>
        <path d="M12 22V8" />
        <path d="M5 12H2a10 10 0 0 0 20 0h-3" />
        <circle cx="12" cy="5" r="3" />
      </>
    ),
  },
  {
    key: "research",
    label: "Research",
    paths: (
      <>
        <rect x="4" y="4" width="16" height="16" rx="2" />
        <rect x="9" y="9" width="6" height="6" />
        <path d="M9 1v3M15 1v3M9 20v3M15 20v3M20 9h3M20 14h3M1 9h3M1 14h3" />
      </>
    ),
  },
];

const navLinkClass = (collapsed: boolean) =>
  ({ isActive }: { isActive: boolean }) =>
    `flex items-center gap-3 rounded-md px-3 py-2.5 text-15 transition-colors ${
      collapsed ? "justify-center" : ""
    } ${
      isActive
        ? "bg-blue-500/15 text-blue-300 ring-1 ring-inset ring-blue-500/30"
        : "text-slate-400 hover:bg-slate-800/60 hover:text-slate-200"
    }`;

function SectionLabel({ collapsed, children }: { collapsed: boolean; children: string }) {
  if (collapsed) return null;
  return (
    <div className="mb-3 px-2.5 text-xs font-semibold uppercase tracking-wide text-slate-500">
      {children}
    </div>
  );
}

// A collapsible track section header (First Flight, Recon, ...). Clicking it
// folds that track's labs away. Lighter and tighter than the top-level
// SectionLabel; deliberately no count - just the label and a disclosure chevron.
function TrackHeader({
  label,
  icon,
  open,
  onToggle,
}: {
  label: string;
  icon?: ReactNode;
  open: boolean;
  onToggle: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onToggle}
      aria-expanded={open}
      className="mb-1.5 mt-4 flex w-full items-center gap-2 px-2.5 text-[11px] font-semibold uppercase tracking-wide text-slate-500 transition-colors first:mt-0 hover:text-slate-300"
    >
      {open ? (
        <ChevronDown size={13} className="shrink-0" />
      ) : (
        <ChevronRight size={13} className="shrink-0" />
      )}
      {icon}
      <span>{label}</span>
    </button>
  );
}

// One instance serves both layouts (so there's a single nav landmark, not two):
// an in-flow collapsible rail on md+, and a full-width off-canvas drawer below md
// (slid in/out by mobileOpen). AppLayout closes the drawer on route change, so
// nav links need no per-link handler; onNavigate backs the explicit close button.
export default function SideNav({
  mobileOpen = false,
  onNavigate,
}: {
  mobileOpen?: boolean;
  onNavigate?: () => void;
}) {
  const narrow = useIsNarrow();
  const { user, logout, avatarVersion } = useAuth();
  const navigate = useNavigate();
  // The lab currently open (route /labs/<slug>), so the collapsed rail can show
  // just that one lab's icon under its track glyph and hide the rest.
  const activeSlug = useLocation().pathname.match(/^\/labs\/([^/]+)/)?.[1] ?? null;
  const [collapsedPref, setCollapsed] = useState(
    () => typeof window !== "undefined" && localStorage.getItem(COLLAPSED_KEY) === "1",
  );
  // Per-track fold state, persisted: which track sections the user has collapsed.
  // A Set of track keys; absent from the set = expanded. On a first visit (no
  // saved entry) we fold every track except First Flight, so a newcomer lands on
  // the onboarding sampler open and the tactic tracks tucked away rather than a
  // long wall of sections. Once the user toggles anything the choice is saved and
  // this default never fires again. Kept separate from the whole-sidebar collapse.
  const [collapsedTracks, setCollapsedTracks] = useState<Set<string>>(() => {
    if (typeof window === "undefined") return new Set();
    const saved = localStorage.getItem(TRACKS_COLLAPSED_KEY);
    if (saved == null) {
      return new Set(TRACKS.filter((t) => t.key !== "first-flight").map((t) => t.key));
    }
    try {
      return new Set(JSON.parse(saved));
    } catch {
      return new Set();
    }
  });
  function toggleTrack(key: string) {
    setCollapsedTracks((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      localStorage.setItem(TRACKS_COLLAPSED_KEY, JSON.stringify([...next]));
      return next;
    });
  }
  // From the collapsed icon rail, a track glyph expands the whole sidebar and
  // makes sure that track is unfolded - so the rail's tactic icons double as
  // "jump into this track" buttons rather than being dead separators.
  function expandToTrack(key: string) {
    setCollapsed(false);
    localStorage.setItem(COLLAPSED_KEY, "0");
    setCollapsedTracks((prev) => {
      if (!prev.has(key)) return prev;
      const next = new Set(prev);
      next.delete(key);
      localStorage.setItem(TRACKS_COLLAPSED_KEY, JSON.stringify([...next]));
      return next;
    });
  }
  // The drawer is always full-width; collapse is a desktop-rail affordance only.
  const collapsed = narrow ? false : collapsedPref;
  const { data: modulesResult, isLoading, isError } = useQuery({
    queryKey: ["modules", "withMeta"],
    queryFn: getModulesWithMeta,
  });
  const modules = modulesResult?.modules;
  // Labs still locked behind progression. Drives the "Encrypted" teaser: it
  // shows only while there's genuinely more to unlock, and disappears once the
  // user has opened the whole catalog (also 0 for admins, who see everything).
  const lockedRemaining = modulesResult?.lockedRemaining ?? 0;
  // React Query keeps the last successful data when a *refetch* fails, so isError
  // can be true while modules still holds a usable list. Gate the sidebar's error
  // and its locked-entry visibility on whether we actually have a list, not on
  // isError alone - otherwise a transient refetch blip flashes "Failed to load
  // labs" directly above the labs that are still listed (and hides the locked
  // "unlock more" entry) even though nothing is really gone.
  const hasModules = (modules?.length ?? 0) > 0;
  // Group the catalog by tactic track, in TRACKS order, preserving the server's
  // curriculum order within each group (modules already arrives sorted). A module
  // whose track isn't one TRACKS knows falls into a trailing "More" group so it
  // can never silently disappear from the list. Empty groups are dropped.
  const allMods = modules ?? [];
  const trackGroups = TRACKS.map((t) => ({
    ...t,
    mods: allMods.filter((m) => m.track === t.key),
  })).filter((g) => g.mods.length > 0);
  const knownTracks = new Set(TRACKS.map((t) => t.key));
  const orphanMods = allMods.filter((m) => !knownTracks.has(m.track ?? ""));
  if (orphanMods.length > 0) {
    trackGroups.push({ key: "other", label: "More", paths: null, mods: orphanMods });
  }

  function toggleCollapsed() {
    setCollapsed((c) => {
      localStorage.setItem(COLLAPSED_KEY, c ? "0" : "1");
      return !c;
    });
  }

  const linkClass = navLinkClass(collapsed);

  // One lab row. Solved labs swap the leading terminal glyph for a green check so
  // completion is obvious at a glance; unsolved rows keep the terminal glyph. The
  // active row's blue highlight stays unchanged.
  const renderLab = (m: NonNullable<typeof modules>[number]) => (
    <li key={m.slug}>
      <NavLink
        to={`/labs/${m.slug}`}
        className={linkClass}
        title={collapsed ? m.displayName : undefined}
      >
        {m.completed ? (
          <Check size={19} className="shrink-0 text-emerald-400" aria-label="solved" />
        ) : (
          <Terminal size={19} className="shrink-0" />
        )}
        {!collapsed && <span className="truncate">{m.displayName}</span>}
      </NavLink>
    </li>
  );

  return (
    <nav
      className={`z-50 flex shrink-0 flex-col border-r border-slate-800 bg-slate-950 fixed inset-y-0 left-0 w-72 transition-transform duration-200 md:static md:translate-x-0 md:transition-[width] ${
        mobileOpen ? "translate-x-0" : "-translate-x-full"
      } ${collapsed ? "md:w-20" : "md:w-72"}`}
    >
      <div
        className={`flex h-20 shrink-0 items-center gap-2 border-b border-slate-800 px-4 ${
          collapsed ? "justify-center px-2" : "justify-between"
        }`}
      >
        {!collapsed && (
          <Link
            to="/"
            className="group flex min-w-0 items-center gap-3 rounded-lg"
            aria-label="K8 Labs home"
          >
            <img src="/logo.svg" alt="" className="h-12 w-auto shrink-0" />
            <h1 className="truncate text-lg font-semibold tracking-tight text-slate-100 transition-colors group-hover:text-white">
              K8 Labs
            </h1>
          </Link>
        )}
        {narrow ? (
          <button
            onClick={onNavigate}
            title="Close menu"
            aria-label="Close menu"
            className="shrink-0 rounded-md p-2.5 text-slate-500 transition-colors hover:bg-slate-800 hover:text-slate-300"
          >
            <X size={19} />
          </button>
        ) : (
          <button
            onClick={toggleCollapsed}
            title={collapsed ? "Expand sidebar" : "Collapse sidebar"}
            aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
            className="shrink-0 rounded-md p-2.5 text-slate-500 transition-colors hover:bg-slate-800 hover:text-slate-300"
          >
            {collapsed ? <PanelLeftOpen size={19} /> : <PanelLeftClose size={19} />}
          </button>
        )}
      </div>

      <div className="flex-1 overflow-y-auto p-4">
        {user?.role === "admin" && (
          <>
            <SectionLabel collapsed={collapsed}>Admin</SectionLabel>
            <ul className="space-y-1">
              <li>
                <NavLink
                  to="/admin"
                  end
                  className={linkClass}
                  title={collapsed ? "Dashboard" : undefined}
                >
                  <LayoutDashboard size={19} className="shrink-0" />
                  {!collapsed && <span className="truncate">Dashboard</span>}
                </NavLink>
              </li>
              <li>
                <NavLink
                  to="/admin/users"
                  className={linkClass}
                  title={collapsed ? "Users" : undefined}
                >
                  <Users size={19} className="shrink-0" />
                  {!collapsed && <span className="truncate">Users</span>}
                </NavLink>
              </li>
              <li>
                <NavLink
                  to="/admin/labs"
                  className={linkClass}
                  title={collapsed ? "Labs" : undefined}
                >
                  <FlaskConical size={19} className="shrink-0" />
                  {!collapsed && <span className="truncate">Labs</span>}
                </NavLink>
              </li>
              <li>
                <NavLink
                  to="/admin/drivers"
                  className={linkClass}
                  title={collapsed ? "Drivers" : undefined}
                >
                  <HardDrive size={19} className="shrink-0" />
                  {!collapsed && <span className="truncate">Drivers</span>}
                </NavLink>
              </li>
            </ul>
            <div className="mt-8" />
          </>
        )}

        <SectionLabel collapsed={collapsed}>General</SectionLabel>
        <ul className="space-y-1">
          <li>
            <NavLink
              to="/settings"
              className={linkClass}
              title={collapsed ? "Settings" : undefined}
            >
              <Settings size={19} className="shrink-0" />
              {!collapsed && <span className="truncate">Settings</span>}
            </NavLink>
          </li>
        </ul>
        <div className="mt-8" />

        <SectionLabel collapsed={collapsed}>Labs</SectionLabel>
        {!collapsed && isLoading && <p className="px-2.5 text-sm text-slate-500">Loading…</p>}
        {!collapsed && isError && !hasModules && (
          <p className="px-2.5 text-sm text-red-400">Failed to load labs</p>
        )}

        {/* "Next up" leaderboard target - reframes the top-of-Labs progress from
            "clear the whole catalog" to "climb the board" (see LeaderboardTarget). */}
        {!collapsed && hasModules && <LeaderboardTarget totalLabs={modules?.length ?? 0} />}

        {/* Collapsed rail: one glyph per track plus only the open lab. Expanded:
            one collapsible section per tactic track, in TRACKS order. Completion
            is shown per row (the green check in renderLab), not as a bucket. */}
        {collapsed ? (
          // Icon-only rail: one glyph per track (a button that expands the
          // sidebar to that track), and the single lab you currently have open
          // shown under its track - not every lab, which was just a wall of
          // identical terminal icons. The active track's glyph brightens so you
          // can see which category you're in at a glance.
          <div className="space-y-1">
            {trackGroups.map(({ key, label, paths, mods }) => {
              const activeLab = mods.find((m) => m.slug === activeSlug);
              return (
                <div key={key}>
                  {paths && (
                    <button
                      type="button"
                      onClick={() => expandToTrack(key)}
                      title={`${label} - expand`}
                      aria-label={`${label} - expand`}
                      className={`flex w-full justify-center rounded-md px-3 py-2.5 transition-colors hover:bg-slate-800/60 hover:text-slate-300 ${
                        activeLab ? "text-slate-300" : "text-slate-600"
                      }`}
                    >
                      <TrackGlyph paths={paths} size={19} />
                    </button>
                  )}
                  {activeLab && <ul className="space-y-1">{renderLab(activeLab)}</ul>}
                </div>
              );
            })}
          </div>
        ) : (
          <>
            {trackGroups.map(({ key, label, paths, mods }) => {
              const open = !collapsedTracks.has(key);
              return (
                <Fragment key={key}>
                  <TrackHeader
                    label={label}
                    icon={paths ? <TrackGlyph paths={paths} size={14} /> : undefined}
                    open={open}
                    onToggle={() => toggleTrack(key)}
                  />
                  {open && <ul className="space-y-1">{mods.map(renderLab)}</ul>}
                </Fragment>
              );
            })}
          </>
        )}

        <ul className="mt-1 space-y-1">
          {/* Locked "Encrypted" entry for regular users: shown while progression
              still has labs left to unlock, so the Labs section signals there's
              more behind the lock rather than looking final. It disappears once
              the user has unlocked the whole catalog (lockedRemaining === 0).
              Routes to /labs/more. Admins see everything, so lockedRemaining is
              always 0 for them and this never renders. */}
          {!isLoading && (hasModules || !isError) && user?.role !== "admin" && lockedRemaining > 0 && (
            <li>
              <NavLink
                to="/labs/more"
                end
                title={collapsed ? "Locked - unlock more labs" : undefined}
                className={({ isActive }) =>
                  `flex items-center gap-3 rounded-md border border-dashed px-3 py-2.5 text-15 transition-colors ${
                    collapsed ? "justify-center" : ""
                  } ${
                    isActive
                      ? "border-blue-500/40 bg-blue-500/10 text-blue-300"
                      : "border-slate-800 text-slate-500 hover:border-slate-700 hover:bg-slate-800/40 hover:text-slate-300"
                  }`
                }
              >
                <Lock size={17} className="shrink-0" />
                {!collapsed && (
                  <span className="flex min-w-0 flex-1 items-center gap-2">
                    <span className="font-mono text-13 uppercase tracking-wider">Encrypted</span>
                    {/* redacted "lab name" bars - it looks like a lab is there,
                        just hidden, which is the point. */}
                    <span className="ml-auto flex items-center gap-1" aria-hidden>
                      <span className="h-1.5 w-6 rounded-full bg-slate-700/70" />
                      <span className="h-1.5 w-3 rounded-full bg-slate-700/50" />
                    </span>
                  </span>
                )}
              </NavLink>
            </li>
          )}
        </ul>
      </div>

      {user && (
        <div className="shrink-0 border-t border-slate-800 p-4">
          <div className={`flex items-center gap-3 ${collapsed ? "justify-center" : ""}`}>
            <Link
              to="/settings"
              title="Settings"
              aria-label="Settings"
              // inline-flex so the box hugs the 36px avatar exactly - an inline
              // <a> inherits line-box height and the focus/hover ring would draw
              // as an oval around it rather than a clean circle.
              className="inline-flex shrink-0 rounded-full ring-offset-2 ring-offset-slate-950 transition
                hover:ring-2 hover:ring-slate-600 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
            >
              <Avatar user={user} size={36} version={avatarVersion} />
            </Link>
            {!collapsed && (
              <div className="min-w-0 flex-1">
                <div className="truncate text-15 font-medium text-slate-200">
                  {user.username}
                </div>
                <div className="truncate text-sm text-slate-500">{user.email}</div>
              </div>
            )}
          </div>
          <button
            onClick={() => {
              // Land on the public "/" home, not /login. Navigate first so we
              // leave the protected route before logout() clears the user -
              // otherwise ProtectedRoute would bounce us to /login?redirect=...
              navigate("/");
              logout();
            }}
            title="Sign out"
            aria-label="Sign out"
            className={`mt-3 flex w-full items-center gap-3 rounded-md px-3 py-2.5 text-15 text-slate-500 transition-colors hover:bg-slate-800/60 hover:text-slate-300 ${
              collapsed ? "justify-center" : ""
            }`}
          >
            <LogOut size={19} className="shrink-0" />
            {!collapsed && <span>Sign out</span>}
          </button>
        </div>
      )}
    </nav>
  );
}
