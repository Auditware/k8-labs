import { useEffect, useRef, useState, type Ref } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { Trophy, Lock, ArrowRight, ChevronLeft, ChevronRight } from "lucide-react";

import { getLeaderboard, type Leaderboard, type LeaderboardEntry } from "../lib/api";
import { useAuth } from "../context/AuthContext";
import { Avatar } from "../components/ui/Avatar";
import { Medal, type Tone } from "../components/ui/Medal";
import { HeroEscapeBackground } from "../components/HeroEscapeBackground";
import { PublicNav } from "../layout/PublicNav";
import { cn } from "../lib/cn";
import { useSeo } from "../lib/seo";

// Public leaderboard. The top ranks show for everyone (signed in or not); the
// full board and "your rank" unlock once you've cleared a lab. The server drives
// the gate - it sends only the leaders with `locked: true` until you're on the
// board - so this page renders whatever it's given and never has to know the
// total operator count. All stats are real (see lib/api getLeaderboard).

// Medal treatment for the top three. The avatar keeps its normal fill; rank
// identity is carried by the ring colour and a corner medal - the same coin+star
// Medal the landing's track-record cards use (components/ui/Medal), so the two
// surfaces read as one instead of this page showing a bare number.
const MEDALS: { ring: string; tone: Tone }[] = [
  { ring: "ring-[#e6c36b]", tone: "gold" },
  { ring: "ring-[#bfd0e0]", tone: "silver" },
  { ring: "ring-[#ce8a55]", tone: "bronze" },
];

// How many compact rows to show per page in the full-board section (the podium's
// top three always stay pinned above it). The full roster now lists every
// operator, so a large board would otherwise run off the page; paging keeps it
// to one screenful at a time.
const BOARD_PAGE_SIZE = 20;

function formatSolve(s: number): string {
  if (s < 3600) {
    const m = Math.floor(s / 60);
    const ss = s % 60;
    return `${m}:${String(ss).padStart(2, "0")}`;
  }
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  return `${h}h ${String(m).padStart(2, "0")}m`;
}

// The distinguishing line under a top-three handle, paired from the server's
// superlative slug and this row's own number. Returns null when there's nothing
// true to say - the card then shows just the provider chip.
function distinctionLine(e: LeaderboardEntry): string | null {
  switch (e.distinction) {
    case "fastest escape":
      return e.fastestSolveSeconds != null
        ? `fastest escape · ${formatSolve(e.fastestSolveSeconds)}`
        : null;
    case "longest streak":
      return e.longestStreakDays > 0 ? `longest streak · ${e.longestStreakDays} days` : null;
    case "most labs cleared":
      return "most labs cleared";
    default:
      return null;
  }
}

// A row only loads its uploaded avatar when the viewer is signed in - the avatar
// endpoint is auth-gated, so for a signed-out visitor we render the initial
// instead of a broken image.
function RankAvatar({
  entry,
  size,
  authed,
  avatarVersion,
}: {
  entry: LeaderboardEntry;
  size: number;
  authed: boolean;
  avatarVersion: number;
}) {
  // A medal is earned by clearing at least one lab - a zero-stat user who only
  // sorts near the top (nobody's cleared anything yet) gets a neutral ring, not
  // gold for nothing.
  const medal = entry.rank <= 3 && entry.labsCompleted > 0 ? MEDALS[entry.rank - 1] : null;
  const user = { id: entry.userId ?? "", username: entry.username, hasAvatar: authed && entry.hasAvatar };
  return (
    <span className="relative inline-flex shrink-0">
      <Avatar
        user={user}
        size={size}
        version={authed && entry.isViewer ? avatarVersion : undefined}
        className={cn("ring-1", medal ? `ring-2 ${medal.ring}` : "ring-hairline")}
      />
      {medal && (
        <span className="absolute -bottom-2 -right-1.5 leading-none">
          <Medal tone={medal.tone} size={24} />
        </span>
      )}
    </span>
  );
}

// The cumulative labs-cleared trend, drawn as a self-scaled sparkline next to the
// labs total (data from entry.labsTrend - real completion history, sampled by the
// server over a window shared across the board, so these curves are comparable
// row to row). The lead row draws it in the accent with a soft fill under the
// line; everyone else in a muted stroke. Renders nothing when there's no shape to
// show (no clears, or every sample equal), so a flat row just shows its number.
function LabsSparkline({ points, accent }: { points: number[] | null | undefined; accent?: boolean }) {
  if (!points || points.length < 2) return null;
  const min = Math.min(...points);
  const max = Math.max(...points);
  const span = max - min;
  if (span === 0) return null;

  const w = 60;
  const h = 30;
  const stepX = w / (points.length - 1);
  // 1px inset top and bottom so the stroke never clips at the extremes.
  const y = (p: number) => h - 1 - ((p - min) / span) * (h - 2);
  const line = points
    .map((p, i) => `${i === 0 ? "M" : "L"}${(i * stepX).toFixed(1)} ${y(p).toFixed(1)}`)
    .join(" ");

  return (
    <svg
      width={w}
      height={h}
      viewBox={`0 0 ${w} ${h}`}
      className={cn("hidden shrink-0 sm:block", accent ? "text-accent" : "text-slate-600")}
      aria-hidden
    >
      {accent && (
        <path d={`${line} L${w} ${h} L0 ${h} Z`} fill="currentColor" fillOpacity={0.13} stroke="none" />
      )}
      <path
        d={line}
        fill="none"
        stroke="currentColor"
        strokeWidth={1.75}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function Stat({ k, v, good }: { k: string; v: string; good?: boolean }) {
  return (
    <div className="text-right">
      <div className={cn("font-mono text-17 font-bold tabular-nums", good ? "text-status-ok" : "text-slate-100")}>
        {v}
      </div>
      <div className="mt-0.5 font-mono text-10 uppercase tracking-wider text-slate-500">{k}</div>
    </div>
  );
}

// The "sensor caught you" tag the eBPF lens raises over the rank-1 row (see the
// HeroEscapeBackground wiring in the page component). Cosmetic - a wink at the
// sensor flagging the top operator as an anomaly.
function AnomalyChip() {
  return (
    <span className="inline-flex shrink-0 items-center gap-1 rounded-full border border-[#ff5a63]/45 bg-[#ff5a63]/10 px-2 py-0.5 font-mono text-10 uppercase tracking-wide text-[#ff8a90]">
      <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-[#ff5a63]" />
      anomaly
    </span>
  );
}

// A top-three "dossier": rank, medal avatar, handle + distinction, three stats.
// cardRef/alert are wired only for the rank-1 row so the hero lens can hit-test
// it and light it up when caught.
function DossierCard({
  entry,
  authed,
  avatarVersion,
  cardRef,
  alert = false,
}: {
  entry: LeaderboardEntry;
  authed: boolean;
  avatarVersion: number;
  cardRef?: Ref<HTMLDivElement>;
  alert?: boolean;
}) {
  // The accent lead treatment is earned too - not applied to a rank-1 row that
  // hasn't cleared anything yet.
  const lead = entry.rank === 1 && entry.labsCompleted > 0;
  const line = distinctionLine(entry);
  // One border-color class only - pick red when caught, else the earned lead
  // accent, else the hairline - so there's no Tailwind source-order conflict.
  const borderCls = alert
    ? "border-[#ff5a63]/60 shadow-[0_0_0_1px_rgba(255,90,99,0.45)]"
    : lead
      ? "border-accent/40"
      : "border-hairline";
  return (
    <div
      ref={cardRef}
      className={cn(
        "flex flex-wrap items-center gap-x-4 gap-y-3 rounded-xl border p-4 transition-colors sm:p-5",
        lead ? "bg-gradient-to-r from-accent/[0.07] to-surface" : "bg-surface",
        borderCls,
      )}
    >
      <div
        className={cn(
          "w-9 text-center font-mono text-26 font-bold tabular-nums",
          lead ? "text-accent" : "text-slate-600",
        )}
      >
        {String(entry.rank).padStart(2, "0")}
      </div>
      <RankAvatar entry={entry} size={48} authed={authed} avatarVersion={avatarVersion} />
      <div className="min-w-[8rem] flex-1">
        <div className="flex items-center gap-2">
          <span className="truncate font-semibold text-slate-100">
            <Link to={`/u/${encodeURIComponent(entry.username)}`} className="transition-colors hover:text-accent">
              {entry.username}
            </Link>
            {entry.isViewer && <span className="ml-1.5 font-mono text-11 text-accent">you</span>}
          </span>
          {alert && <AnomalyChip />}
        </div>
        {line && <div className="mt-1 truncate font-mono text-11 text-slate-500">{line}</div>}
      </div>
      <div className="ml-auto flex items-center gap-5 sm:gap-6">
        <div className="flex items-center gap-2.5 sm:gap-3">
          <LabsSparkline points={entry.labsTrend} accent={lead} />
          <Stat k="labs" v={String(entry.labsCompleted)} />
        </div>
        <Stat
          k="fastest"
          v={entry.fastestSolveSeconds != null ? formatSolve(entry.fastestSolveSeconds) : "-"}
          good={entry.distinction === "fastest escape"}
        />
        <Stat k="streak" v={`${entry.longestStreakDays}d`} />
      </div>
    </div>
  );
}

// One redacted row in the encrypted (locked) region. Shares the dossier's shape
// so it reads as the same kind of entry, just sealed. Ranks run 04, 05, ·· - the
// last is indeterminate so the tail never implies an exact count.
function RedactedRow({ n }: { n: number }) {
  const rank = n === 2 ? "··" : String(n + 4).padStart(2, "0");
  const handleW = ["w-32", "w-40", "w-24"][n];
  return (
    <div className="flex items-center gap-4 rounded-xl border border-hairline bg-surface p-4 sm:p-5">
      <div className="w-9 text-center font-mono text-22 font-bold text-[#2b3a4c]">{rank}</div>
      <div
        className="grid h-11 w-11 shrink-0 place-items-center rounded-full text-[#33465a]"
        style={{ background: "repeating-linear-gradient(135deg,#182534 0 6px,#141f2c 6px 12px)" }}
      >
        <Lock size={15} aria-hidden />
      </div>
      <div className={cn("h-3 rounded bg-[#223041]", handleW)} />
      <div className="ml-auto flex items-center gap-6">
        {[0, 1, 2].map((j) => (
          <div key={j} className={cn("h-3 rounded bg-[#223041]", j === 1 ? "w-10" : "w-5")} />
        ))}
      </div>
    </div>
  );
}

// The locked tail: redacted rows dissolving into a "decrypt" prompt. Framed as
// security, not a paywall. CTA depends on how far the viewer is from unlocking -
// a signed-out visitor must sign in first; a signed-in one just needs a clear.
function EncryptedRegion({ authed }: { authed: boolean }) {
  const navigate = useNavigate();
  return (
    <div className="mt-4">
      <div className="mb-3 flex items-center gap-2 border-t border-dashed border-hairline pt-4 font-mono text-10 uppercase tracking-[0.14em] text-slate-600">
        <Lock size={12} aria-hidden /> encrypted
      </div>
      <div
        className="flex flex-col gap-3"
        style={{ maskImage: "linear-gradient(180deg,#000 0,#000 42%,transparent 100%)", WebkitMaskImage: "linear-gradient(180deg,#000 0,#000 42%,transparent 100%)" }}
        aria-hidden
      >
        {[0, 1, 2].map((i) => (
          <RedactedRow key={i} n={i} />
        ))}
      </div>
      <div className="mt-3 flex flex-wrap items-center justify-between gap-4 rounded-xl border border-hairline-strong bg-gradient-to-r from-accent/[0.05] to-surface p-4 sm:p-5">
        <div className="flex items-center gap-3.5">
          <div className="grid h-9 w-9 shrink-0 place-items-center rounded-xl bg-accent/10 text-accent ring-1 ring-inset ring-accent/25">
            <Lock size={18} aria-hidden />
          </div>
          <div>
            <h3 className="font-semibold text-slate-100">Solve a lab to decrypt the leaderboard</h3>
          </div>
        </div>
        <button
          onClick={() => navigate(authed ? "/labs" : "/login")}
          className="inline-flex h-9 items-center gap-2 rounded-lg bg-accent px-4 text-13 font-semibold text-[#042c53] transition-colors hover:bg-[#7fb4f5] focus:outline-none focus-visible:ring-2 focus-visible:ring-accent"
        >
          {authed ? "Start a lab" : "Sign in"}
          <ArrowRight size={15} aria-hidden />
        </button>
      </div>
    </div>
  );
}

// A compact ranked row for positions past the podium, shown only in the unlocked
// (full-board) view.
function CompactRow({
  entry,
  authed,
  avatarVersion,
}: {
  entry: LeaderboardEntry;
  authed: boolean;
  avatarVersion: number;
}) {
  return (
    <div
      className={cn(
        "flex items-center gap-4 rounded-lg border px-4 py-3",
        entry.isViewer ? "border-accent/40 bg-accent/[0.06]" : "border-hairline bg-surface",
      )}
    >
      <div className="w-8 text-center font-mono text-15 font-bold tabular-nums text-slate-500">
        {entry.rank}
      </div>
      <RankAvatar entry={entry} size={34} authed={authed} avatarVersion={avatarVersion} />
      <div className="min-w-0 flex-1">
        <div className="truncate text-sm font-medium text-slate-200">
          <Link to={`/u/${encodeURIComponent(entry.username)}`} className="transition-colors hover:text-accent">
            {entry.username}
          </Link>
          {entry.isViewer && <span className="ml-1.5 font-mono text-11 text-accent">you</span>}
        </div>
      </div>
      <div className="flex items-center gap-6 font-mono text-13 tabular-nums text-slate-300">
        <span className="w-6 text-right">{entry.labsCompleted}</span>
        <span className="hidden w-12 text-right sm:inline">
          {entry.fastestSolveSeconds != null ? formatSolve(entry.fastestSolveSeconds) : "-"}
        </span>
        <span className="hidden w-8 text-right sm:inline">{entry.longestStreakDays}d</span>
      </div>
    </div>
  );
}

function BoardSkeleton() {
  return (
    <div className="flex flex-col gap-3" aria-hidden>
      {[0, 1, 2].map((i) => (
        <div key={i} className="flex items-center gap-4 rounded-xl border border-hairline bg-surface p-4 sm:p-5">
          <div className="h-6 w-6 animate-pulse rounded bg-slate-800" />
          <div className="h-12 w-12 animate-pulse rounded-full bg-slate-800" />
          <div className="flex-1 space-y-2">
            <div className="h-3.5 w-40 animate-pulse rounded bg-slate-800" />
            <div className="h-3 w-24 animate-pulse rounded bg-slate-800" />
          </div>
          <div className="hidden gap-6 sm:flex">
            {[0, 1, 2].map((j) => (
              <div key={j} className="h-8 w-10 animate-pulse rounded bg-slate-800" />
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}

function Body({
  data,
  authed,
  avatarVersion,
  alertCardRef,
  sensorAlert,
}: {
  data: Leaderboard;
  authed: boolean;
  avatarVersion: number;
  alertCardRef: Ref<HTMLDivElement>;
  sensorAlert: boolean;
}) {
  const { entries } = data;

  // Everything past the podium is the paginated full board. Start on the page
  // that holds the viewer's own row (if they're down the list), so a signed-in
  // operator lands on themselves rather than always at the top.
  const rest = entries.slice(3);
  const [page, setPage] = useState(() => {
    const vi = rest.findIndex((e) => e.isViewer);
    return vi >= 0 ? Math.floor(vi / BOARD_PAGE_SIZE) : 0;
  });
  const pageCount = Math.max(1, Math.ceil(rest.length / BOARD_PAGE_SIZE));
  const safePage = Math.min(page, pageCount - 1);
  // Keep the control state in range if the board shrank under us (a refetch can
  // drop the count below the current page's start).
  useEffect(() => {
    if (safePage !== page) setPage(safePage);
  }, [safePage, page]);

  if (entries.length === 0) {
    // Reached only when there are no operators at all - the board now lists every
    // registered user, so any roster fills in above instead of this state.
    return (
      <div className="flex flex-col items-center gap-4 rounded-xl border border-hairline bg-surface px-6 py-16 text-center">
        <div className="grid h-12 w-12 place-items-center rounded-2xl bg-accent/10 text-accent">
          <Trophy size={22} aria-hidden />
        </div>
        <div>
          <h3 className="text-17 font-semibold text-slate-100">The board is open</h3>
          <p className="mx-auto mt-1 max-w-sm text-sm text-slate-400">
            No users yet. Sign in to open the board and take the top rank.
          </p>
        </div>
        <Link
          to={authed ? "/labs" : "/login"}
          className="inline-flex h-9 items-center gap-2 rounded-lg bg-accent px-4 text-13 font-semibold text-[#042c53] transition-colors hover:bg-[#7fb4f5]"
        >
          {authed ? "Start a lab" : "Sign in"}
          <ArrowRight size={15} aria-hidden />
        </Link>
      </div>
    );
  }

  const podium = entries.slice(0, 3);
  const pageRows = rest.slice(safePage * BOARD_PAGE_SIZE, safePage * BOARD_PAGE_SIZE + BOARD_PAGE_SIZE);

  return (
    <>
      <div className="flex flex-col gap-3">
        {podium.map((e) => (
          <DossierCard
            key={e.rank}
            entry={e}
            authed={authed}
            avatarVersion={avatarVersion}
            cardRef={e.rank === 1 ? alertCardRef : undefined}
            alert={e.rank === 1 && sensorAlert}
          />
        ))}
      </div>

      {data.locked ? (
        <EncryptedRegion authed={authed} />
      ) : (
        rest.length > 0 && (
          <div className="mt-6">
            <div className="mb-3 flex items-center justify-between font-mono text-10 uppercase tracking-[0.14em] text-slate-600">
              <span>full board</span>
              <span className="hidden gap-6 sm:flex">
                <span className="w-6 text-right">labs</span>
                <span className="w-12 text-right">fastest</span>
                <span className="w-8 text-right">streak</span>
              </span>
            </div>
            <div className="flex flex-col gap-2">
              {pageRows.map((e) => (
                <CompactRow key={e.rank} entry={e} authed={authed} avatarVersion={avatarVersion} />
              ))}
            </div>

            {rest.length > BOARD_PAGE_SIZE && (
              <div className="mt-4 flex items-center justify-between font-mono text-11 text-slate-500">
                <span>
                  ranks{" "}
                  <span className="text-slate-300 tabular-nums">
                    {safePage * BOARD_PAGE_SIZE + 4}-{Math.min((safePage + 1) * BOARD_PAGE_SIZE + 3, entries.length)}
                  </span>{" "}
                  of <span className="text-slate-300 tabular-nums">{entries.length}</span>
                </span>
                <div className="flex items-center gap-2">
                  <button
                    type="button"
                    disabled={safePage === 0}
                    onClick={() => setPage((p) => Math.max(0, p - 1))}
                    className="inline-flex h-8 items-center gap-1 rounded-lg border border-hairline bg-surface px-3 text-slate-300 transition-colors hover:border-hairline-strong hover:text-slate-100 disabled:cursor-not-allowed disabled:opacity-40"
                  >
                    <ChevronLeft size={14} aria-hidden /> Prev
                  </button>
                  <span className="tabular-nums text-slate-500">
                    {safePage + 1} / {pageCount}
                  </span>
                  <button
                    type="button"
                    disabled={safePage >= pageCount - 1}
                    onClick={() => setPage((p) => Math.min(pageCount - 1, p + 1))}
                    className="inline-flex h-8 items-center gap-1 rounded-lg border border-hairline bg-surface px-3 text-slate-300 transition-colors hover:border-hairline-strong hover:text-slate-100 disabled:cursor-not-allowed disabled:opacity-40"
                  >
                    Next <ChevronRight size={14} aria-hidden />
                  </button>
                </div>
              </div>
            )}
          </div>
        )
      )}
    </>
  );
}

export default function LeaderboardsPage() {
  useSeo({
    title: "Leaderboard - K8 Labs",
    description:
      "The K8 Labs leaderboard: fastest roots, longest streaks, and top-percentile ranks across the Kubernetes attack & defense labs.",
    path: "/leaderboard",
  });
  const { isAuthenticated, avatarVersion } = useAuth();
  const query = useQuery({
    queryKey: ["leaderboard"],
    queryFn: getLeaderboard,
    refetchOnWindowFocus: false,
  });

  // The avatar-load gate keys off the payload's own view of auth so it always
  // agrees with the rows it's rendering.
  const authed = query.data?.viewer.authenticated ?? isAuthenticated;

  // The eBPF sensor "catching" the top operator: the ambient hero lens hit-tests
  // the rank-1 dossier card, and when it passes over it both the lens and the row
  // flip to an alert look. Toggles only on lens enter/leave, so this state is cheap.
  const alertCardRef = useRef<HTMLDivElement>(null);
  const [sensorAlert, setSensorAlert] = useState(false);

  return (
    <div className="relative min-h-screen overflow-x-hidden bg-slate-950 text-slate-100">
      {/* Same ambient escape-grid as the landing, so arriving here from the
          landing nav feels like the same surface rather than a separate page. */}
      <HeroEscapeBackground
        className="pointer-events-none fixed inset-0 h-full w-full"
        sensorTargetRef={alertCardRef}
        onSensorAlert={setSensorAlert}
      />
      <PublicNav />

      <main className="relative z-10 mx-auto max-w-6xl px-4 pb-20 pt-44 sm:px-6 2xl:max-w-[86rem]">
        <div className="mb-8 flex flex-col items-center gap-3 text-center">
          {/* Same understated label treatment as the landing hero's
              "Hands-on Kubernetes attack & defense", so the two pages read as one
              surface. Kept an h1 for the page's heading semantics. */}
          <h1 className="font-mono text-[14.3px] uppercase tracking-[0.18em] text-accent/85">Leaderboard</h1>
          {query.data?.viewer.rank != null && (
            <div className="rounded-lg border border-hairline bg-surface px-3.5 py-1.5">
              <span className="font-mono text-10 uppercase tracking-wider text-slate-500">your rank </span>
              <span className="font-mono text-15 font-bold tabular-nums text-accent">#{query.data.viewer.rank}</span>
            </div>
          )}
        </div>

        {query.isPending ? (
          <BoardSkeleton />
        ) : query.isError ? (
          <div className="rounded-xl border border-status-danger/30 bg-status-danger/5 px-5 py-8 text-center">
            <p className="text-sm text-status-danger">Couldn't load the leaderboard. Try again shortly.</p>
          </div>
        ) : (
          <Body
            data={query.data}
            authed={authed}
            avatarVersion={avatarVersion}
            alertCardRef={alertCardRef}
            sensorAlert={sensorAlert}
          />
        )}
      </main>
    </div>
  );
}
