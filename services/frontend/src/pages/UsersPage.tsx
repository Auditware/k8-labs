import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowDown, ArrowUp, ArrowUpDown, Ban, Check, ChevronDown, ChevronLeft, ChevronRight, Pencil, Search, ShieldCheck, Trash2, Undo2, X } from "lucide-react";
import { Page, PageSection } from "../layout/Page";
import { Button } from "../components/ui/Form";
import { Banner } from "../components/ui/Banner";
import { timeAgo } from "../lib/format";
import {
  getAdminUsers,
  getModules,
  grantLab,
  revokeLab,
  setUserBlocked,
  setUserQuota,
  deleteUser,
  type AdminUser,
  type UserUsage,
  type LiveUsage,
  type Module,
} from "../lib/api";
import { useAuth } from "../context/AuthContext";
import { palette } from "../lib/tokens";

// Admin user management. Registration is open (anyone can sign up with Google
// or GitHub), but a new account lands here with zero grants and can see and
// start nothing until an admin ticks a lab on. This page is that switchboard.
//
// It's a UX surface only - every action here is enforced server-side by
// requireAdmin plus the entitlement checks in createSession/listModules. A
// user who forged their way to this page would still get 403s.
// How many users to show per page. The prod roster outgrew a single scroll -
// the last row's controls (block/delete, the grant menu) ended up off the bottom
// of the page - so the list is paged. 10 keeps every row's actions reachable
// without scrolling.
const PAGE_SIZE = 10;

// Columns the roster can be sorted by. null = the server's own order
// (most-recently-solved, then most-recently-registered), the default.
type SortKey = "user" | "role" | "access" | "usage" | "status";

export default function UsersPage() {
  const queryClient = useQueryClient();
  const { user: currentUser } = useAuth();
  const [page, setPage] = useState(0);
  const [search, setSearch] = useState("");
  const [sortKey, setSortKey] = useState<SortKey | null>(null);
  const [sortDir, setSortDir] = useState<"asc" | "desc">("asc");

  // Clicking a header sorts by it; clicking the active one flips direction.
  const toggleSort = (key: SortKey) => {
    setPage(0);
    if (sortKey === key) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortKey(key);
      setSortDir("asc");
    }
  };

  // Poll: the usage column now carries a live "running now" figure that accrues
  // while a lab is open, so a static fetch would show a frozen number. 15s
  // matches the topology dashboard's cadence and is well under the shortest
  // session, so a running lab is always reflected.
  const usersQuery = useQuery({
    queryKey: ["admin", "users"],
    queryFn: getAdminUsers,
    refetchInterval: 15_000,
  });
  const modulesQuery = useQuery({ queryKey: ["modules"], queryFn: getModules });

  // Every mutation refetches the user list rather than patching it locally:
  // revoke and block have server-side side effects (tearing down sessions), so
  // the server's view is the only one worth trusting afterwards.
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["admin", "users"] });

  const grant = useMutation({
    mutationFn: ({ userId, slug }: { userId: string; slug: string }) => grantLab(userId, slug),
    onSuccess: invalidate,
  });
  const revoke = useMutation({
    mutationFn: ({ userId, slug }: { userId: string; slug: string }) => revokeLab(userId, slug),
    onSuccess: invalidate,
  });
  const block = useMutation({
    mutationFn: ({ userId, blocked }: { userId: string; blocked: boolean }) =>
      setUserBlocked(userId, blocked),
    onSuccess: invalidate,
  });
  const remove = useMutation({
    mutationFn: ({ userId }: { userId: string }) => deleteUser(userId),
    onSuccess: invalidate,
  });
  const setQuota = useMutation({
    mutationFn: ({ userId, gibMinutes }: { userId: string; gibMinutes: number | null }) =>
      setUserQuota(userId, gibMinutes),
    onSuccess: invalidate,
  });

  const pendingError = grant.error || revoke.error || block.error || remove.error || setQuota.error;

  const allUsers = usersQuery.data?.users ?? [];

  // 24h usage figure the Usage column shows: finished consumption in the window
  // plus what's accruing live right now. Reused as the sort value.
  const usageValue = (u: (typeof allUsers)[number]) =>
    (u.usage?.windowGibMinutes ?? 0) + (u.live?.gibMinutes ?? 0);

  // Filter by the free-text query (username or email), then sort. The default
  // (sortKey null) preserves the server's order. A stable, case-insensitive
  // compare; direction flips the result.
  const users = useMemo(() => {
    const q = search.trim().toLowerCase();
    let list = allUsers;
    if (q) {
      list = list.filter(
        (u) =>
          u.username.toLowerCase().includes(q) || u.email.toLowerCase().includes(q),
      );
    }
    if (!sortKey) return list;
    const dir = sortDir === "asc" ? 1 : -1;
    const cmp = (a: (typeof allUsers)[number], b: (typeof allUsers)[number]) => {
      switch (sortKey) {
        case "user":
          return a.username.localeCompare(b.username);
        case "role":
          return a.role.localeCompare(b.role);
        case "access":
          return (a.entitled?.length ?? 0) - (b.entitled?.length ?? 0);
        case "usage":
          return usageValue(a) - usageValue(b);
        case "status":
          return Number(a.blocked) - Number(b.blocked);
        default:
          return 0;
      }
    };
    // Copy before sort so the query cache array is never mutated in place.
    return [...list].sort((a, b) => dir * cmp(a, b));
  }, [allUsers, search, sortKey, sortDir]);

  // Page the (filtered, sorted) roster. Clamp the page in case the list shrank
  // under us - a delete, a poll, or a tighter search can drop the count below
  // the current page's start.
  const pageCount = Math.max(1, Math.ceil(users.length / PAGE_SIZE));
  const safePage = Math.min(page, pageCount - 1);
  const pageUsers = useMemo(
    () => users.slice(safePage * PAGE_SIZE, safePage * PAGE_SIZE + PAGE_SIZE),
    [users, safePage],
  );
  // If the clamp moved us, sync the state so the controls agree with what's shown.
  useEffect(() => {
    if (safePage !== page) setPage(safePage);
  }, [safePage, page]);

  if (usersQuery.isLoading) {
    return <p className="p-6 text-sm text-slate-500">Loading users…</p>;
  }
  if (usersQuery.isError) {
    return <p className="p-6 text-sm text-red-400">Failed to load users.</p>;
  }

  const modules = modulesQuery.data ?? [];
  // Per-user daily allowance (GiB-minutes); 0 = quota disabled. The usage column
  // draws each user's 24h consumption as a fraction of this, so the bar means
  // "how close to being throttled" rather than "heaviest right now."
  const dailyQuota = usersQuery.data?.dailyQuotaGibMinutes ?? 0;

  return (
    <Page title="Users">
      <PageSection title="Accounts and lab access">
        {pendingError && (
          <Banner tone="error" className="mb-4">
            {(pendingError as Error).message}
          </Banner>
        )}

        <div className="mb-4 flex flex-wrap items-center gap-3">
          <div className="relative min-w-[16rem] flex-1">
            <Search
              size={15}
              className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-slate-500"
            />
            <input
              type="search"
              value={search}
              onChange={(e) => {
                setSearch(e.target.value);
                setPage(0);
              }}
              placeholder="Search by username or email…"
              className="w-full rounded-lg border border-slate-800 bg-slate-900/60 py-2 pl-9 pr-3 text-sm text-slate-200 placeholder:text-slate-600 focus:border-slate-600 focus:outline-none"
            />
          </div>
          <span className="text-xs text-slate-500">
            {users.length} {users.length === 1 ? "user" : "users"}
            {search.trim() && ` matching "${search.trim()}"`}
          </span>
        </div>

        <div className="overflow-x-auto rounded-lg border border-slate-800">
          <table className="w-full min-w-[46rem] text-left text-sm">
            <thead className="bg-slate-900/60 text-xs uppercase tracking-wide text-slate-500">
              <tr>
                <SortHeader label="User" col="user" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />
                <SortHeader label="Role" col="role" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />
                <SortHeader label="Lab access" col="access" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />
                <SortHeader label="Usage (24h)" col="usage" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />
                <SortHeader label="Status" col="status" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />
                <th className="px-4 py-3 font-semibold" />
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800">
              {pageUsers.map((u) => (
                <UserRow
                  key={u.id}
                  user={u}
                  modules={modules}
                  dailyQuota={dailyQuota}
                  isSelf={u.id === currentUser?.id}
                  busy={grant.isPending || revoke.isPending || block.isPending || remove.isPending || setQuota.isPending}
                  onSetQuota={(gibMinutes) => setQuota.mutate({ userId: u.id, gibMinutes })}
                  onToggleGrant={(slug, granted) =>
                    granted
                      ? revoke.mutate({ userId: u.id, slug })
                      : grant.mutate({ userId: u.id, slug })
                  }
                  onToggleBlock={() => block.mutate({ userId: u.id, blocked: !u.blocked })}
                  onDelete={() => remove.mutate({ userId: u.id })}
                />
              ))}
            </tbody>
          </table>
        </div>

        {users.length === 0 && (
          <p className="mt-4 text-sm text-slate-500">
            {search.trim()
              ? `No users match "${search.trim()}".`
              : "No users have registered yet."}
          </p>
        )}

        {users.length > PAGE_SIZE && (
          <div className="mt-4 flex items-center justify-between text-xs text-slate-500">
            <span>
              Showing{" "}
              <span className="text-slate-300">
                {safePage * PAGE_SIZE + 1}-{Math.min((safePage + 1) * PAGE_SIZE, users.length)}
              </span>{" "}
              of <span className="text-slate-300">{users.length}</span>
            </span>
            <div className="flex items-center gap-2">
              <Button
                variant="ghost"
                size="sm"
                type="button"
                disabled={safePage === 0}
                onClick={() => setPage((p) => Math.max(0, p - 1))}
                className="disabled:cursor-not-allowed disabled:opacity-40"
              >
                <ChevronLeft size={14} /> Prev
              </Button>
              <span className="tabular-nums">
                Page {safePage + 1} / {pageCount}
              </span>
              <Button
                variant="ghost"
                size="sm"
                type="button"
                disabled={safePage >= pageCount - 1}
                onClick={() => setPage((p) => Math.min(pageCount - 1, p + 1))}
                className="disabled:cursor-not-allowed disabled:opacity-40"
              >
                Next <ChevronRight size={14} />
              </Button>
            </div>
          </div>
        )}
      </PageSection>
    </Page>
  );
}

// SortHeader is a clickable table header. It shows a neutral up/down glyph when
// inactive and a filled arrow for the active sort direction, so the current sort
// is obvious at a glance and every column reads as sortable.
function SortHeader({
  label,
  col,
  sortKey,
  sortDir,
  onSort,
}: {
  label: string;
  col: SortKey;
  sortKey: SortKey | null;
  sortDir: "asc" | "desc";
  onSort: (col: SortKey) => void;
}) {
  const active = sortKey === col;
  const Icon = !active ? ArrowUpDown : sortDir === "asc" ? ArrowUp : ArrowDown;
  return (
    <th className="px-4 py-3 font-semibold">
      <button
        type="button"
        onClick={() => onSort(col)}
        aria-label={`Sort by ${label}`}
        aria-sort={active ? (sortDir === "asc" ? "ascending" : "descending") : "none"}
        className={`group -mx-1 inline-flex items-center gap-1.5 rounded px-1 py-0.5 uppercase tracking-wide hover:text-slate-300 ${
          active ? "text-slate-300" : ""
        }`}
      >
        {label}
        <Icon
          size={13}
          className={active ? "text-blue-400" : "text-slate-600 group-hover:text-slate-400"}
        />
      </button>
    </th>
  );
}

// LabAccessMenu is the per-user grant editor: a compact "N of M labs" trigger
// that opens a checklist of the whole catalog. It replaced an inline row of a
// toggle chip per lab, which turned into a tall column once the catalog passed a
// dozen labs. The panel is portaled to <body> with fixed positioning because the
// table lives in an overflow-x-auto wrapper (which also clips vertically), so an
// in-flow absolute panel would be cut off.
function LabAccessMenu({
  modules,
  grants,
  completed,
  entitled,
  busy,
  onToggleGrant,
  readOnly = false,
}: {
  modules: Module[];
  grants: string[];
  completed: string[];
  entitled: string[];
  busy: boolean;
  onToggleGrant: (slug: string, granted: boolean) => void;
  // Admins bypass the grants table (see accounts.HasGrant), so their access is
  // not editable - but they still solve labs, and an admin wanting to see their
  // own captures had nowhere to look. readOnly drops the grant checkboxes and
  // keeps the panel purely as a solved-labs view.
  readOnly?: boolean;
}) {
  const [open, setOpen] = useState(false);
  // Either `top` (opening downward) or `bottom` (flipped up, anchored above the
  // trigger) is set, never both - see place() for which side is chosen.
  const [pos, setPos] = useState<{ top?: number; bottom?: number; left: number; width: number; maxHeight: number } | null>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);

  const grantedSet = new Set(grants);
  const solvedSet = new Set(completed);
  const count = grantedSet.size;
  // What the user can actually access: explicit grants plus the free progression
  // unlocks (tier-0 for everyone, plus each tier the previous one opened). The
  // summary counts this so a user on the free first lab reads as "1 of N", not
  // "No labs", and each row below marks access-without-a-grant distinctly from an
  // explicit grant, so the count and the checkmarks agree.
  const entitledSet = new Set(entitled);
  const accessCount = entitledSet.size;
  const total = modules.length;

  // Position the fixed panel under the trigger. Kept out of the click handler and
  // run in a layout effect instead, so opening is a single boolean toggle (two
  // setStates in one handler was racing and leaving the panel unmounted) and the
  // rect is read after the DOM has settled.
  const place = useCallback(() => {
    const btn = buttonRef.current;
    if (!btn) return;
    const r = btn.getBoundingClientRect();
    const width = 288; // 18rem
    const gap = 4;
    const margin = 12; // keep the panel this far from the viewport edge
    const left = Math.max(8, Math.min(r.left, window.innerWidth - width - 8));
    const spaceBelow = window.innerHeight - r.bottom - margin;
    const spaceAbove = r.top - margin;
    // Open downward by default; flip up when below is cramped and there's more
    // room above. Without this the last row on a page pushed the checklist off
    // the bottom of the screen. maxHeight is clamped to the chosen side so the
    // panel always fits and scrolls internally rather than overflowing.
    const openUp = spaceBelow < 260 && spaceAbove > spaceBelow;
    const maxHeight = Math.max(0, Math.min(openUp ? spaceAbove : spaceBelow, 360));
    if (openUp) {
      setPos({ bottom: window.innerHeight - r.top + gap, left, width, maxHeight });
    } else {
      setPos({ top: r.bottom + gap, left, width, maxHeight });
    }
  }, []);

  useLayoutEffect(() => {
    if (open) place();
  }, [open, place]);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    const onClickOutside = (e: MouseEvent) => {
      const t = e.target as Node;
      if (panelRef.current && !panelRef.current.contains(t) && buttonRef.current && !buttonRef.current.contains(t)) {
        setOpen(false);
      }
    };
    // The trigger sits in an overflow-x scroll container, so a scroll or resize
    // moves it. Re-place the panel to stay glued to it rather than closing.
    document.addEventListener("keydown", onKey);
    document.addEventListener("mousedown", onClickOutside);
    window.addEventListener("resize", place);
    window.addEventListener("scroll", place, true); // capture: catch nested scrollers
    return () => {
      document.removeEventListener("keydown", onKey);
      document.removeEventListener("mousedown", onClickOutside);
      window.removeEventListener("resize", place);
      window.removeEventListener("scroll", place, true);
    };
  }, [open, place]);

  const summary = readOnly
    ? total === 0
      ? "No labs in catalog"
      : "All labs (admin)"
    : total === 0
      ? "No labs in catalog"
      : accessCount === 0
        ? "No labs"
        : accessCount === total
          ? `All ${total} labs`
          : `${accessCount} of ${total} labs`;

  // The trigger only lights up (emerald) when it's an editable grant control
  // with grants set, matching the checkboxes. The read-only admin view stays
  // neutral - "N solved" is informational, not a state worth highlighting.
  const active = !readOnly && count > 0;
  const activeClass = "bg-emerald-500/15 text-emerald-300 ring-1 ring-inset ring-emerald-500/30 hover:bg-emerald-500/25";

  return (
    <>
      <button
        ref={buttonRef}
        type="button"
        disabled={total === 0}
        onClick={() => setOpen((o) => !o)}
        aria-haspopup="menu"
        aria-expanded={open}
        className={`inline-flex items-center gap-1.5 whitespace-nowrap rounded-md px-2.5 py-1.5 text-xs transition-colors disabled:opacity-50 ${
          active ? activeClass : "bg-slate-800/60 text-slate-300 ring-1 ring-inset ring-slate-700 hover:bg-slate-800"
        }`}
      >
        <span>{summary}</span>
        <ChevronDown size={13} className={`transition-transform ${open ? "rotate-180" : ""}`} />
      </button>

      {open &&
        pos &&
        createPortal(
          <div
            ref={panelRef}
            role="menu"
            style={{ position: "fixed", top: pos.top, bottom: pos.bottom, left: pos.left, width: pos.width, maxHeight: pos.maxHeight }}
            className="z-50 overflow-y-auto rounded-lg border border-hairline bg-surface p-1.5 shadow-2xl"
          >
            {modules.map((m) => {
              const granted = grantedSet.has(m.slug);
              const solved = solvedSet.has(m.slug);
              // Solved marker: the checkbox is access (can run), this plain
              // "solved" label is achievement (flag captured) - kept as quiet
              // muted text, not a badge. Shared by both row variants.
              const solvedMark = solved && (
                <span title="Solved - flag captured" className="shrink-0 text-slate-500">
                  solved
                </span>
              );

              // Read-only (admin): no grant toggle, just the solved view. The row
              // is a static div, and an unsolved lab reads dimmer than a solved one.
              if (readOnly) {
                return (
                  <div
                    key={m.slug}
                    role="menuitem"
                    className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-xs"
                  >
                    <span className={`min-w-0 flex-1 truncate ${solved ? "text-slate-200" : "text-slate-500"}`}>
                      {m.displayName}
                    </span>
                    {solvedMark}
                  </div>
                );
              }

              // Accessible without an explicit grant: the free first lab (tier 0)
              // or a progression unlock. Shown as a muted (not emerald) check so
              // the row agrees with the "N labs" count, while staying visibly
              // distinct from a manual grant. Still clickable - the admin can pin
              // an explicit grant on top so access survives a progression change.
              const autoAccess = !granted && entitledSet.has(m.slug);
              const title = granted
                ? "Explicitly granted - click to revoke"
                : autoAccess
                  ? "Open automatically (free first lab or a progression unlock) - click to also set an explicit grant"
                  : "No access - click to grant";
              return (
                <button
                  key={m.slug}
                  type="button"
                  role="menuitemcheckbox"
                  aria-checked={granted}
                  title={title}
                  disabled={busy}
                  onClick={() => onToggleGrant(m.slug, granted)}
                  className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-xs text-slate-300 transition-colors hover:bg-slate-800 disabled:opacity-50"
                >
                  <span
                    className={`flex h-4 w-4 shrink-0 items-center justify-center rounded border ${
                      granted
                        ? "border-emerald-500/60 bg-emerald-500/20 text-emerald-300"
                        : autoAccess
                          ? "border-slate-500 bg-slate-600/30 text-slate-400"
                          : "border-slate-600 text-transparent"
                    }`}
                  >
                    <Check size={11} />
                  </span>
                  <span className="min-w-0 flex-1 truncate">{m.displayName}</span>
                  {autoAccess && (
                    <span title="Open automatically, not a manual grant" className="shrink-0 text-11 text-slate-500">
                      auto
                    </span>
                  )}
                  {solvedMark}
                </button>
              );
            })}
          </div>,
          document.body,
        )}
    </>
  );
}

function UserRow({
  user,
  modules,
  dailyQuota,
  isSelf,
  busy,
  onToggleGrant,
  onToggleBlock,
  onDelete,
  onSetQuota,
}: {
  user: AdminUser;
  modules: Module[];
  dailyQuota: number;
  isSelf: boolean;
  busy: boolean;
  onToggleGrant: (slug: string, granted: boolean) => void;
  onToggleBlock: () => void;
  onDelete: () => void;
  onSetQuota: (gibMinutes: number | null) => void;
}) {
  const isAdmin = user.role === "admin";
  // Effective cap for this user's usage bar: their own override if set, else the
  // global default. `??` (not `||`) so an override of 0 - "unlimited" - is
  // respected rather than falling through to the global.
  const effectiveQuota = user.quotaOverrideGibMinutes ?? dailyQuota;

  return (
    <tr className={user.blocked ? "bg-red-500/5" : undefined}>
      <td className="px-4 py-3 align-top">
        <div className="font-medium text-slate-200">{user.username}</div>
        <div className="text-slate-500">{user.email}</div>
        <div className="mt-0.5 text-xs text-slate-600">
          via {user.oauthProvider}
          {user.lastSolvedAt && (
            <span title={`Last solved a lab ${new Date(user.lastSolvedAt).toLocaleString()}`}>
              {" · "}solved {timeAgo(user.lastSolvedAt)}
            </span>
          )}
        </div>
      </td>

      <td className="px-4 py-3 align-top">
        {isAdmin ? (
          <span className="inline-flex items-center gap-1 rounded-md bg-blue-500/15 px-2 py-1 text-xs font-medium text-blue-300 ring-1 ring-inset ring-blue-500/30">
            <ShieldCheck size={13} /> admin
          </span>
        ) : (
          <span className="text-xs text-slate-500">user</span>
        )}
      </td>

      <td className="px-4 py-3 align-top">
        {/* A dropdown checklist rather than an inline chip cloud: the catalog has
            grown past a dozen labs, and rendering every grant toggle inline made
            the row a very tall column. Collapsed to one compact control that opens
            on demand. For admins it renders read-only (they bypass grants, so the
            toggles would do nothing) but still shows which labs they've solved. */}
        <LabAccessMenu
          modules={modules}
          grants={user.grants}
          completed={user.completed}
          entitled={user.entitled}
          busy={busy}
          onToggleGrant={onToggleGrant}
          readOnly={isAdmin}
        />
      </td>

      <td className="px-4 py-3 align-top">
        <UsageCell
          usage={user.usage}
          live={user.live}
          solved={user.completed.length}
          effectiveQuota={effectiveQuota}
          override={user.quotaOverrideGibMinutes}
          globalDefault={dailyQuota}
          busy={busy}
          onSetQuota={onSetQuota}
        />
      </td>

      <td className="px-4 py-3 align-top">
        {user.blocked ? (
          <span className="inline-flex items-center gap-1 rounded-md bg-red-500/15 px-2 py-1 text-xs font-medium text-red-300 ring-1 ring-inset ring-red-500/30">
            <Ban size={13} /> blocked
          </span>
        ) : (
          <span className="text-xs text-emerald-400">active</span>
        )}
      </td>

      <td className="px-4 py-3 align-top">
        <div className="flex items-center justify-end gap-2">
          <Button
            variant="ghost"
            size="sm"
            type="button"
            disabled={busy || isSelf}
            onClick={onToggleBlock}
            // Blocking yourself locks you out of this very page - the backend
            // rejects it too (400), this just doesn't offer it.
            title={isSelf ? "You can't block your own account" : undefined}
            className="disabled:cursor-not-allowed"
          >
            {user.blocked ? (
              <>
                <Undo2 size={13} /> Unblock
              </>
            ) : (
              <>
                <Ban size={13} /> Block
              </>
            )}
          </Button>

          <Button
            variant="danger"
            size="sm"
            type="button"
            disabled={busy || isSelf}
            // Deleting is irreversible and drops the grants with it, so it asks
            // first - unlike block, which is a one-click toggle precisely
            // because it can be undone. The backend refuses self-deletion too.
            onClick={() => {
              if (
                window.confirm(
                  `Delete ${user.username} (${user.email})?\n\n` +
                    "This permanently removes the account and its lab access, and " +
                    "stops any lab they're running. It can't be undone - use Block " +
                    "instead to suspend them reversibly.",
                )
              ) {
                onDelete();
              }
            }}
            title={isSelf ? "You can't delete your own account" : "Delete this user"}
            className="disabled:cursor-not-allowed"
          >
            <Trash2 size={13} /> Delete
          </Button>
        </div>
      </td>
    </tr>
  );
}

// One user's consumption. A live "using N now" line appears first when the user
// has a session running, because that is the state an operator is most likely
// reacting to; the historical 24h figure (what the daily quota acts on) and the
// lifetime total follow as context.
//
// Live matters specifically because a running session has no finished-usage row
// yet: without this line a student mid-lab reads as "-", which is the confusing
// case - it looks like nothing is being counted while they actively consume.
//
// Shown as GiB-minutes rather than a "credits" score deliberately: the unit is
// self-explaining (memory held x minutes held) and does not silently commit the
// product to an exchange rate. An eBPF lab accrues ~1.67x a normal one per
// minute because it genuinely reserves that much more.
//
// The 24h figure is ALWAYS drawn as a bar - a number alone was the whole thing
// this column was meant to avoid. Usage fills toward the daily quota, drawn as a
// draggable orange limit line; the fill is coloured by proximity to that line
// (calm until ~70%, amber past it, red at/over), or a neutral accent when the
// user has no cap. See usageScale for how the bar's ceiling is chosen.
function usageBarColor(pct: number): string {
  if (pct >= 100) return palette.danger;
  if (pct >= 70) return palette.warn;
  return palette.ok;
}

const clamp = (n: number, lo: number, hi: number) => Math.max(lo, Math.min(hi, n));

// A stable, round ceiling for the usage bar: the larger of usage and the current
// quota, with ~60% headroom so the limit can be dragged up as well as down,
// rounded to a multiple of 60 GiB-min (~a lab and a bit) and floored so a fresh
// cell still has a usable drag range.
function usageScale(win: number, quota: number): number {
  return Math.ceil((Math.max(win, quota, 90) * 1.6) / 60) * 60;
}

// The usage cell: a bar showing 24h consumption, with the daily quota drawn as an
// orange limit line. Clicking the pencil makes that line draggable - drag it to
// set the cap (release to save), drag to the far right for unlimited, arrow keys
// to nudge. The bar, its edit control, and the used/limit line share one fixed
// width so the two rows align.
function UsageCell({
  usage,
  live,
  solved,
  effectiveQuota,
  override,
  globalDefault,
  busy,
  onSetQuota,
}: {
  usage?: UserUsage;
  live?: LiveUsage;
  // Labs this user has solved (captured the flag). Shown as an at-a-glance stat
  // in this cell; the per-lab breakdown lives in the grants menu's solved flags.
  solved: number;
  effectiveQuota: number;
  override?: number;
  globalDefault: number;
  busy: boolean;
  onSetQuota: (gibMinutes: number | null) => void;
}) {
  const barRef = useRef<HTMLDivElement>(null);
  const [editing, setEditing] = useState(false);
  const [drag, setDrag] = useState<number | null>(null); // live limit (GiB-min) while editing

  const hasLive = !!live && live.sessions > 0;
  const hasOverride = override !== undefined;
  const win = usage?.windowGibMinutes ?? 0;
  const hasQuota = effectiveQuota > 0;

  // Freeze the scale for the duration of an edit so the bar can't rescale under
  // the handle if the table refetches mid-drag.
  const scaleRef = useRef(0);
  if (!editing) scaleRef.current = usageScale(win, hasQuota ? effectiveQuota : 0);
  const scale = scaleRef.current;

  // Focus the bar on entering edit so the arrow-key nudge works immediately.
  useEffect(() => {
    if (editing) barRef.current?.focus();
  }, [editing]);

  const unlimited = editing && drag !== null && drag >= scale * 0.985;
  const shownLimit = editing ? drag : hasQuota ? effectiveQuota : null;
  const usedPct = scale > 0 ? clamp((win / scale) * 100, 0, 100) : 0;
  const limitPct = shownLimit != null && !unlimited ? clamp((shownLimit / scale) * 100, 0, 100) : null;

  // Colour usage by proximity to whatever limit is in force (turns amber/red as
  // the fill nears the line); neutral accent when there is no cap.
  const refLimit = editing ? (unlimited ? 0 : drag ?? 0) : effectiveQuota;
  const usedColor = refLimit > 0 ? usageBarColor((win / refLimit) * 100) : palette.accent;

  const limitText = editing
    ? unlimited
      ? "∞"
      : formatGibMinutes(drag ?? 0)
    : hasQuota
      ? formatGibMinutes(effectiveQuota)
      : null;

  const quotaTitle = hasOverride
    ? override === 0
      ? "Daily quota: unlimited (custom) - click to edit"
      : `Daily quota: ${formatGibMinutes(override)} GiB-min (custom) - click to edit`
    : globalDefault > 0
      ? `Daily quota: ${formatGibMinutes(globalDefault)} GiB-min (default) - click to set an override`
      : "No daily quota (default) - click to set one";

  const valueAt = (clientX: number): number => {
    const el = barRef.current;
    if (!el) return 0;
    const r = el.getBoundingClientRect();
    return clamp((clientX - r.left) / r.width, 0, 1) * scale;
  };
  const startEdit = () => {
    setDrag(hasQuota ? effectiveQuota : Math.round(scale * 0.5));
    setEditing(true);
  };
  const cancel = () => {
    setEditing(false);
    setDrag(null);
  };
  const commit = (v: number) => {
    // Backend: 0 = unlimited, positive = cap. The far right releases as
    // unlimited; anything else is a positive cap floored at 5 so a near-zero
    // drag can't collapse to 0 and silently mean "unlimited".
    onSetQuota(v >= scale * 0.985 ? 0 : Math.max(5, Math.round(v)));
    setEditing(false);
    setDrag(null);
  };

  const onPointerDown = (e: React.PointerEvent<HTMLDivElement>) => {
    if (!editing) return;
    e.currentTarget.setPointerCapture(e.pointerId);
    setDrag(valueAt(e.clientX));
  };
  const onPointerMove = (e: React.PointerEvent<HTMLDivElement>) => {
    if (!editing || e.buttons === 0) return;
    setDrag(valueAt(e.clientX));
  };
  const onPointerUp = (e: React.PointerEvent<HTMLDivElement>) => {
    if (!editing) return;
    e.currentTarget.releasePointerCapture(e.pointerId);
    if (drag !== null) commit(drag);
  };
  const onKeyDown = (e: React.KeyboardEvent<HTMLDivElement>) => {
    if (!editing) return;
    if (e.key === "Escape") cancel();
    else if (e.key === "Enter" && drag !== null) commit(drag);
    else if (e.key === "ArrowLeft" || e.key === "ArrowRight") {
      e.preventDefault();
      const step = (e.shiftKey ? 30 : 6) * (e.key === "ArrowRight" ? 1 : -1);
      setDrag((d) => clamp((d ?? effectiveQuota) + step, 0, scale));
    }
  };

  return (
    <div className="w-44 text-xs">
      {hasLive && (
        <div className="mb-1 flex items-center gap-1 font-medium text-emerald-300">
          <span className="inline-block h-1.5 w-1.5 animate-pulse rounded-full bg-emerald-400" />
          {live!.sessions} running · {formatBytes(live!.reservedBytes)}
        </div>
      )}

      <div className="flex items-center gap-2">
        <div
          ref={barRef}
          tabIndex={editing ? 0 : -1}
          onPointerDown={onPointerDown}
          onPointerMove={onPointerMove}
          onPointerUp={onPointerUp}
          onKeyDown={onKeyDown}
          className={`relative h-2.5 flex-1 rounded-full bg-slate-800 outline-none ${editing ? "cursor-ew-resize ring-1 ring-slate-600" : ""}`}
          title={
            editing
              ? "Drag the orange line to set the daily quota; release to save (far right = unlimited)"
              : hasQuota
                ? `${formatGibMinutes(win)} of ${formatGibMinutes(effectiveQuota)} GiB-min quota`
                : `${formatGibMinutes(win)} GiB-min used - no quota`
          }
        >
          <div className="absolute inset-y-0 left-0 rounded-full" style={{ width: `${usedPct}%`, background: usedColor }} />
          {limitPct != null && (
            <div className="absolute inset-y-0" style={{ left: `${limitPct}%` }}>
              <div className="absolute inset-y-[-3px] -left-px w-0.5 rounded" style={{ background: palette.limit }} />
              {editing && (
                <>
                  <div
                    className="absolute left-1/2 top-1/2 h-3.5 w-3.5 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-slate-950"
                    style={{ background: palette.limit }}
                  />
                  <div
                    className="absolute -top-5 left-1/2 -translate-x-1/2 whitespace-nowrap rounded px-1 py-0.5 font-mono text-10 font-medium text-slate-950"
                    style={{ background: palette.limit }}
                  >
                    {limitText}
                  </div>
                </>
              )}
            </div>
          )}
          {editing && unlimited && (
            <div
              className="absolute right-0 top-1/2 h-3.5 w-3.5 -translate-y-1/2 translate-x-1/2 rounded-full border-2 border-slate-950"
              style={{ background: palette.limit }}
            />
          )}
        </div>

        {editing ? (
          <div className="flex shrink-0 items-center">
            {hasOverride && (
              <button
                type="button"
                disabled={busy}
                onClick={() => {
                  onSetQuota(null); // clear override -> inherit the global default
                  cancel();
                }}
                title="Clear override - inherit the global default"
                className="rounded p-0.5 text-slate-500 transition-colors hover:text-slate-300 disabled:opacity-40"
              >
                <Undo2 size={13} />
              </button>
            )}
            <button
              type="button"
              onClick={cancel}
              title="Cancel"
              className="rounded p-0.5 text-slate-500 transition-colors hover:text-slate-300"
            >
              <X size={13} />
            </button>
          </div>
        ) : (
          <button
            type="button"
            disabled={busy}
            onClick={startEdit}
            title={quotaTitle}
            className={`shrink-0 rounded p-0.5 transition-colors hover:text-slate-300 disabled:opacity-40 ${
              hasOverride ? "text-accent" : "text-slate-600"
            }`}
          >
            <Pencil size={12} />
          </button>
        )}
      </div>

      {(usage || editing) && (
        <div className="mt-1 font-mono text-11 text-slate-400">
          {formatGibMinutes(win)}
          {limitText != null && <span className="text-slate-500">/{limitText}</span>} GiB-min
          {usage && (
            <span className="text-slate-600">
              {" · "}
              {usage.windowSessions} {usage.windowSessions === 1 ? "session" : "sessions"}
            </span>
          )}
        </div>
      )}

      {/* Trailing stats: lifetime consumption and labs solved share one line
          (each part optional), so "solved" never takes a dedicated row of its
          own - it just rides the "all time" line with a middot separator. */}
      {!editing && ((usage && usage.totalSessions > usage.windowSessions) || solved > 0) && (
        <div className="text-10 text-slate-600">
          {usage && usage.totalSessions > usage.windowSessions && `${formatGibMinutes(usage.totalGibMinutes)} all time`}
          {usage && usage.totalSessions > usage.windowSessions && solved > 0 && " · "}
          {solved > 0 && `${solved} solved`}
        </div>
      )}

      {!usage && hasLive && !editing && <div className="mt-1 text-slate-600">0 sessions</div>}
    </div>
  );
}

// Reserved memory as GiB/MiB. Live sessions reserve whole-GiB-ish amounts
// (1.5 / 2.5), so one decimal of GiB is the readable unit.
function formatBytes(b: number): string {
  const gib = b / 1024 ** 3;
  if (gib >= 1) return `${gib.toFixed(1)} GiB`;
  return `${Math.round(b / 1024 ** 2)} MiB`;
}

// Whole numbers past 10 - a tenth of a GiB-minute is noise at the scale these
// are read at (a full 30-minute normal lab is ~45).
function formatGibMinutes(v: number): string {
  if (v === 0) return "0";
  if (v < 10) return v.toFixed(1);
  return Math.round(v).toLocaleString();
}
