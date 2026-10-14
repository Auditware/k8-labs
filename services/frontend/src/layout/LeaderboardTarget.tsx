import { useCallback, useEffect, useLayoutEffect, useRef, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { useQuery } from "@tanstack/react-query";
import { ExternalLink } from "lucide-react";
import { getLeaderboard, type LeaderboardEntry } from "../lib/api";
import { formatSolve } from "../lib/format";
import { Avatar } from "../components/ui/Avatar";
import { Medal, type Tone } from "../components/ui/Medal";

// The "Next up" target at the top of the sidebar's Labs section. It reframes
// progress as "climb the board": the operator one rank above you (or #2 when you
// lead), with a bar whose segments are LABS - one per lab in the catalog. Your
// cleared labs fill green from the left; slate marks the rival's own lab; the
// gap between you glows amber ONLY once they're within striking range (see
// NEAR_LABS), so the amber ignites as you close in rather than being a wall of
// caution when you're far behind. Everything else stays dim.
//
// Nothing here navigates except the avatar, which links to that operator's
// public profile. The widget as a whole is intentionally not a link.

type Slot = "gold" | "dim" | "rival" | "you";

const SLOT_CLASS: Record<Slot, string> = {
  gold: "bg-amber-400", // the gap: labs between you and the rival
  dim: "bg-slate-700", //  catalog beyond the rival, not the immediate target
  rival: "bg-slate-400", // the rival's own lab (the marker segment)
  you: "bg-emerald-400", // your cleared labs
};

// The bar is a fixed number of segments, intentionally - it's a RELATIVE picture
// of the climb (your share vs the rival's share of the board), not a lab-by-lab
// ruler. 8 keeps it legible in the narrow sidebar however large the catalog
// grows; markers are placed proportionally into these segments (see Ladder), and
// the exact lab counts live in the caption.
const MAX_SEGMENTS = 8;

// Amber ("gold") is a proximity cue, not a ruler. It only lights up once the
// rival is within this many labs of you; beyond that the gap reads as dim. So a
// far-behind or not-yet-on-board player sees a calm bar instead of a wall of
// amber, and the amber "ignites" as they close into striking range.
const NEAR_LABS = 3;

// Same ring/tone map the leaderboard uses for its top three, so an operator's
// medal looks identical in the hover preview and on the board.
const MEDALS: { ring: string; tone: Tone }[] = [
  { ring: "ring-[#e6c36b]", tone: "gold" },
  { ring: "ring-[#bfd0e0]", tone: "silver" },
  { ring: "ring-[#ce8a55]", tone: "bronze" },
];

// The distinction label only - the numeric value it carries on the board
// (fastest time, streak days) is already shown in the stat tiles below, so
// repeating it here would be duplication.
function distinctionLine(e: LeaderboardEntry): string | null {
  switch (e.distinction) {
    case "fastest escape":
      return e.fastestSolveSeconds != null ? "fastest escape" : null;
    case "longest streak":
      return e.longestStreakDays > 0 ? "longest streak" : null;
    case "most labs cleared":
      return "most labs cleared";
    default:
      return null;
  }
}

// The board's stat cell: mono value over a tiny uppercase mono label.
function Stat({ k, v, good }: { k: string; v: string; good?: boolean }) {
  return (
    <div className="text-center">
      <div className={`font-mono text-17 font-bold tabular-nums ${good ? "text-status-ok" : "text-slate-100"}`}>
        {v}
      </div>
      <div className="mt-0.5 font-mono text-10 uppercase tracking-wider text-slate-500">{k}</div>
    </div>
  );
}

// The medal-ringed avatar the board gives its top three (a plain hairline ring
// otherwise), reproduced so the same operator reads identically in both places.
function MedalAvatar({ entry, size }: { entry: LeaderboardEntry; size: number }) {
  const medal = entry.rank <= 3 && entry.labsCompleted > 0 ? MEDALS[entry.rank - 1] : null;
  const user = { id: entry.userId ?? "", username: entry.username, hasAvatar: entry.hasAvatar };
  return (
    <span className="relative inline-flex shrink-0">
      <Avatar user={user} size={size} className={medal ? `ring-2 ${medal.ring}` : "ring-1 ring-hairline"} />
      {medal && (
        <span className="absolute -bottom-2 -right-1.5 leading-none">
          <Medal tone={medal.tone} size={20} />
        </span>
      )}
    </span>
  );
}

// The avatar is a hover/tap trigger, not a link. Hovering (or tapping, for touch)
// opens a preview card of the operator; the CARD itself is the link to their
// public profile, opened in a new tab so the sidebar isn't navigated away. The
// card renders through a portal with position:fixed so the sidebar's overflow
// doesn't clip it, glued to the trigger and re-placed on scroll/resize.
function OperatorHoverCard({ entry }: { entry: LeaderboardEntry }) {
  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState<{ top: number; left: number } | null>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const cardRef = useRef<HTMLAnchorElement>(null);
  const closeTimer = useRef<number | null>(null);
  const CARD_W = 264;

  const avatarUser = { id: entry.userId ?? "", username: entry.username, hasAvatar: entry.hasAvatar };
  const line = distinctionLine(entry);
  // The board gives the rank-1 operator an accent "lead" treatment, but only once
  // they've actually cleared a lab - match that here.
  const lead = entry.rank === 1 && entry.labsCompleted > 0;

  const place = useCallback(() => {
    const el = triggerRef.current;
    if (!el) return;
    const r = el.getBoundingClientRect();
    const gap = 10;
    const margin = 12;
    // Open to the right of the avatar (the sidebar hugs the left edge); flip left
    // only if the card would run off the right of the viewport.
    let left = r.right + gap;
    if (left + CARD_W > window.innerWidth - margin) left = Math.max(margin, r.left - gap - CARD_W);
    const top = Math.min(Math.max(margin, r.top - 8), window.innerHeight - margin - 170);
    setPos({ top, left });
  }, []);

  useLayoutEffect(() => {
    if (open) place();
  }, [open, place]);

  const cancelClose = () => {
    if (closeTimer.current) {
      window.clearTimeout(closeTimer.current);
      closeTimer.current = null;
    }
  };
  const scheduleClose = () => {
    cancelClose();
    closeTimer.current = window.setTimeout(() => setOpen(false), 120);
  };
  useEffect(() => () => cancelClose(), []);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    const onDown = (e: MouseEvent) => {
      const t = e.target as Node;
      if (cardRef.current?.contains(t) || triggerRef.current?.contains(t)) return;
      setOpen(false);
    };
    window.addEventListener("scroll", place, true);
    window.addEventListener("resize", place);
    document.addEventListener("keydown", onKey);
    document.addEventListener("mousedown", onDown);
    return () => {
      window.removeEventListener("scroll", place, true);
      window.removeEventListener("resize", place);
      document.removeEventListener("keydown", onKey);
      document.removeEventListener("mousedown", onDown);
    };
  }, [open, place]);

  return (
    <>
      <button
        ref={triggerRef}
        type="button"
        aria-label={`${entry.username} - preview profile`}
        onMouseEnter={() => {
          cancelClose();
          setOpen(true);
        }}
        onMouseLeave={scheduleClose}
        onFocus={() => setOpen(true)}
        onBlur={scheduleClose}
        onClick={() => setOpen(true)}
        className="inline-flex rounded-full ring-offset-1 ring-offset-slate-950 transition hover:ring-2 hover:ring-slate-500 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
      >
        <Avatar user={avatarUser} size={18} className="ring-1 ring-slate-600" />
      </button>

      {open &&
        pos &&
        createPortal(
          <a
            ref={cardRef}
            href={`/u/${encodeURIComponent(entry.username)}`}
            target="_blank"
            rel="noopener noreferrer"
            onMouseEnter={cancelClose}
            onMouseLeave={scheduleClose}
            style={{ position: "fixed", top: pos.top, left: pos.left, width: CARD_W }}
            // Solid bg-surface (opaque #0b1120), not the board's translucent accent
            // gradient - as a floating popover it has no page behind it, so a
            // see-through background would let underlying text bleed in. The #1
            // "lead" cue is kept as the accent border instead of the gradient.
            className={`group z-[60] block rounded-xl border bg-surface p-3.5 shadow-2xl transition-colors ${
              lead ? "border-accent/40" : "border-hairline hover:border-hairline-strong"
            }`}
          >
            <div className="flex items-center gap-3">
              <MedalAvatar entry={entry} size={40} />
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-1.5">
                  <span className="truncate font-semibold text-slate-100 transition-colors group-hover:text-accent">
                    {entry.username}
                  </span>
                  {entry.isViewer && <span className="font-mono text-11 text-accent">you</span>}
                  <ExternalLink
                    size={12}
                    className="ml-auto shrink-0 text-slate-600 transition-colors group-hover:text-accent"
                  />
                </div>
                {line && <div className="mt-0.5 truncate font-mono text-11 text-slate-500">{line}</div>}
              </div>
            </div>
            <div className="mt-3 grid grid-cols-3 gap-2 border-t border-hairline pt-3">
              <Stat k="labs" v={String(entry.labsCompleted)} />
              <Stat
                k="fastest"
                v={entry.fastestSolveSeconds != null ? formatSolve(entry.fastestSolveSeconds) : "-"}
                good={entry.distinction === "fastest escape"}
              />
              <Stat k="streak" v={`${entry.longestStreakDays}d`} />
            </div>
          </a>,
          document.body,
        )}
    </>
  );
}

// One head-to-head line shared by every state: "You're N labs ahead of / behind
// X", or "Tied with X" when they're level on labs (the board then splits the tie
// by fastest solve, which the sidebar doesn't try to spell out).
function gapLine(viewerLabs: number, other: LeaderboardEntry): ReactNode {
  // The rank belongs to the OTHER operator (the spot you're chasing / defending
  // against), shown right before their name.
  const name = (
    <span className="text-slate-300">
      #{other.rank} {other.username}
    </span>
  );
  const diff = viewerLabs - other.labsCompleted;
  if (diff === 0) return <>Tied with {name}</>;
  const n = Math.abs(diff);
  const labs = `${n} lab${n === 1 ? "" : "s"}`;
  return diff > 0 ? <>You're {labs} ahead of {name}</> : <>You're {labs} behind {name}</>;
}

// The bar: one segment per lab in the catalog. Your cleared labs fill green;
// the marked operator's own lab (otherLabs - 1) is slate and carries the avatar
// + arrow; labs between the two glow gold only when the rival is within
// NEAR_LABS (else dim); everything past the rival stays dim. otherLabs null (no
// operator to mark, e.g. an empty board) draws a plain fill with no marker.
function Ladder({
  total,
  mine,
  otherLabs,
  avatar,
}: {
  total: number;
  mine: number;
  otherLabs: number | null;
  avatar: ReactNode;
}) {
  // Segments are PROPORTIONAL to the board, not one-per-lab. The catalog (16+)
  // outgrows the fixed segment budget, and mapping absolute lab counts straight
  // to segment indices squashed the whole climb into the last box or two - a
  // 6-lab gap that only starts at box 6 has nowhere left to open up. Instead
  // place each marker at its SHARE of the bar (round(labs / denom * n)), so the
  // gap reads as a proportion of the board however large the catalog grows. denom
  // reaches past the catalog when the rival has cleared more than the viewer can
  // see, so a runaway rival still lands on the bar rather than clamping off the
  // end. Exact counts live in the caption; the bar is a relative picture, by
  // design (MAX_SEGMENTS stays 8 on purpose - see its note).
  const denom = Math.max(total, mine, otherLabs ?? 0, 1);
  const n = Math.min(denom, MAX_SEGMENTS);
  const fill = (labs: number) => Math.min(Math.round((labs / denom) * n), n);
  // Green boxes: your share of the board. At least one once you've cleared a lab
  // so progress is always visible; zero only when you've cleared nothing.
  const mineFill = mine > 0 ? Math.min(Math.max(fill(mine), 1), n) : 0;
  // Your frontier: the last green box. A "you-are-here" caret (emerald ▲) sits
  // under it, mirroring the rival's ▼ above, so the bar shows both positions.
  // With nothing cleared you pin to the leftmost block rather than hiding.
  const frontier = mineFill > 0 ? Math.min(mineFill - 1, n - 1) : 0;
  // The rival's marker sits on their share of the bar. When they're genuinely
  // ahead, force it at least one box past your frontier so proportional rounding
  // can never collapse the two markers onto the same segment ("ahead but rendered
  // level"). When YOU'RE ahead (the #1 chaser view) their marker sits to your
  // left, untouched.
  const rivalShare = otherLabs != null ? Math.max(fill(otherLabs) - 1, 0) : -1;
  const markAt =
    otherLabs != null
      ? Math.min(otherLabs > mine ? Math.max(rivalShare, frontier + 1) : rivalShare, n - 1)
      : -1;
  // Amber stays a proximity cue in TRUE lab units (unscaled): it lights only when
  // the rival is within NEAR_LABS actual labs, so it ignites as you really close
  // in, independent of how the bar is scaled. Everything from your frontier up to
  // the rival marker is the gap.
  const near = otherLabs != null && otherLabs - mine > 0 && otherLabs - mine <= NEAR_LABS;
  const slotAt = (i: number): Slot => {
    if (i === markAt) return "rival";
    if (i < mineFill) return "you";
    if (markAt >= 0 && i < markAt) return near ? "gold" : "dim";
    return "dim";
  };
  const segs = Array.from({ length: n });
  return (
    <div>
      <div className="mb-px flex h-5 gap-[2px]">
        {segs.map((_, i) => (
          <span key={i} className="flex flex-1 items-center justify-center">
            {i === markAt ? avatar : null}
          </span>
        ))}
      </div>
      <div className="mb-[3px] flex h-[9px] gap-[2px]" aria-hidden>
        {segs.map((_, i) => (
          <span key={i} className="flex flex-1 items-center justify-center">
            {i === markAt ? <span className="text-[9px] leading-none text-emerald-400">▼</span> : null}
          </span>
        ))}
      </div>
      <div className="mb-[2px] flex items-center gap-[2px]" aria-hidden>
        {segs.map((_, i) => {
          const s = slotAt(i);
          return (
            <span key={i} className={`flex-1 rounded-full ${SLOT_CLASS[s]} ${s === "you" ? "h-[7px]" : "h-[5px]"}`} />
          );
        })}
      </div>
      <div className="flex h-[9px] gap-[2px]" aria-hidden>
        {segs.map((_, i) => (
          <span key={i} className="flex flex-1 items-center justify-center">
            {i === frontier ? <span className="text-[9px] leading-none text-emerald-400">▲</span> : null}
          </span>
        ))}
      </div>
    </div>
  );
}

// No header at all - just the ladder and caption, tucked under the Labs heading.
function Frame({ children }: { children: ReactNode }) {
  return <div className="-mt-1.5 mb-3 px-2.5">{children}</div>;
}

// The caption line - the head-to-head sentence. The rank shown belongs to the
// other operator and is rendered inline before their name (see gapLine), so the
// caption needs no separate rank chrome.
function CaptionRow({ children }: { children: ReactNode }) {
  return <div className="mt-2 truncate text-[11px] text-slate-400">{children}</div>;
}

export function LeaderboardTarget({ totalLabs }: { totalLabs: number }) {
  const { data } = useQuery({ queryKey: ["leaderboard"], queryFn: getLeaderboard });
  // Guard the whole payload shape, not just presence - a non-critical sidebar
  // widget must never crash the app if the board query ever returns an
  // unexpected body. Renders nothing until there's an authenticated viewer.
  if (!data || !data.viewer?.authenticated) return null;

  const { entries, viewer, nextTarget } = data;
  const mine = viewer.labsCompleted ?? 0;

  // The nearest operator strictly ahead of you, computed by the backend over the
  // FULL board (not just the revealed top ranks). So even a not-yet-on-the-board
  // viewer chases the closest runner to overtake - the lowest-ranked operator
  // with more labs than them - rather than the far-off #1.
  if (nextTarget) {
    return (
      <Frame>
        <Ladder
          total={totalLabs}
          mine={mine}
          otherLabs={nextTarget.labsCompleted}
          avatar={<OperatorHoverCard entry={nextTarget} />}
        />
        <CaptionRow>{gapLine(mine, nextTarget)}</CaptionRow>
      </Frame>
    );
  }

  // Nobody ahead. If you hold #1, turn around - show #2 (the chaser) and how far
  // ahead of them you are.
  if (viewer.rank === 1) {
    const chaser = entries.find((e) => e.rank === 2) ?? null;
    const lead = chaser ? mine - chaser.labsCompleted : null;
    return (
      <Frame>
        <Ladder
          total={totalLabs}
          mine={mine}
          otherLabs={chaser ? chaser.labsCompleted : null}
          avatar={chaser ? <OperatorHoverCard entry={chaser} /> : null}
        />
        <CaptionRow>
          {!chaser ? (
            "No challengers yet"
          ) : lead != null && lead > 0 ? (
            <>
              You're {lead} lab{lead === 1 ? "" : "s"} ahead of{" "}
              <span className="text-slate-300">
                #{chaser.rank} {chaser.username}
              </span>
            </>
          ) : (
            <>
              Holding #1 over{" "}
              <span className="text-slate-300">
                #{chaser.rank} {chaser.username}
              </span>
            </>
          )}
        </CaptionRow>
      </Frame>
    );
  }

  // No one strictly ahead and you're not #1 (empty board, or everyone level with
  // you): just your progress, and a nudge toward the next clear.
  return (
    <Frame>
      <Ladder total={totalLabs} mine={mine} otherLabs={null} avatar={null} />
      <div className="mt-2 truncate text-[11px] text-slate-400">
        {viewer.hasClearedLab ? "Climb the board" : "Solve a lab to join the board"}
      </div>
    </Frame>
  );
}
