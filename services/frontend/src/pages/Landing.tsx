import { useEffect, useMemo, useRef, useState } from "react";
import type { PointerEvent as ReactPointerEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { ArrowRight, Trophy } from "lucide-react";
import Footer from "../components/Footer";
import { HeroEscapeBackground } from "../components/HeroEscapeBackground";
import { ActivityHeatmap } from "../components/ActivityHeatmap";
import { useAuth } from "../context/AuthContext";
import { Avatar } from "../components/ui/Avatar";
import { Medal, type Tone } from "../components/ui/Medal";
import { ServerIssueBadge } from "../components/ServerIssueBadge";
import { getShowcaseOperators, publicAvatarUrl, type PublicProfile } from "../lib/api";
import { formatSolve, joinedAgo, topPercentile } from "../lib/format";
import { useSeo } from "../lib/seo";

// The logged-out marketing landing (state A). Full-bleed, its own scroll - it
// deliberately does NOT mount the app shell; App routes an authenticated visitor
// straight past this to the lab home. See services/frontend landing spec.
//
// A magnetic scroll-deck (scroll-snap y-mandatory), four full-viewport slides,
// each with its own treatment: cinematic clip, typographic mission, a stacked
// deck of researcher track-records, and an editable "fake terminal" closer. Both
// CTAs and the nav sign-in navigate to /login. Under prefers-reduced-motion the
// clip freezes, the terminal spinner stills, and snap relaxes.
//
// The hero clip is bundled same-origin under public/media (allowed by the prod
// CSP's default-src 'self'). It's captured at 1508x952, so on a Retina display
// at large sizes it's upscaled and can look soft - the honest fix is a higher-
// resolution recording, not CSS.

const CTA_LABEL = "See if you can even get in";

function reducedMotion() {
  return (
    typeof window !== "undefined" &&
    window.matchMedia?.("(prefers-reduced-motion: reduce)")?.matches === true
  );
}

// The hero clip is ~2.6MB. On an explicit Save-Data request or a 2g-class
// connection, don't autoplay/eager-load it - the poster still shows.
function savePower() {
  const c =
    typeof navigator !== "undefined"
      ? (navigator as { connection?: { saveData?: boolean; effectiveType?: string } }).connection
      : undefined;
  return !!c && (c.saveData === true || /(?:^|-)2g$/.test(c.effectiveType || ""));
}

function Cta({ onClick, label = CTA_LABEL }: { onClick: () => void; label?: string }) {
  return (
    <button
      onClick={onClick}
      className="inline-flex items-center gap-2 rounded-[10px] bg-accent px-5 py-3 text-15 font-semibold text-slate-950 shadow-[0_8px_30px_-10px_rgba(106,166,242,0.6)] transition-transform hover:-translate-y-px motion-reduce:transform-none"
    >
      {label} <ArrowRight size={16} />
    </button>
  );
}

// ── One operator's track-record card, from real profile data ─────────────────
const MEDAL_TONES: Tone[] = ["gold", "silver", "bronze"];

function CardStat({ v, k }: { v: string; k: string }) {
  return (
    <div>
      <div className="font-mono text-[16px] text-slate-100">{v}</div>
      <div className="mt-0.5 text-10 text-slate-500">{k}</div>
    </div>
  );
}

// A real operator, drawn from the public showcase (top-3 of the live leaderboard).
// Every number here is real: stats and rank from the board, the activity grid
// from actual finished sessions. On the front (interactive) card the handle links
// to the full /u/:username profile; back cards render it as plain text so a click
// there flips the deck instead of navigating.
function ProfileCard({ p, front }: { p: PublicProfile; front: boolean }) {
  const tone = p.rank <= 3 && p.labsCompleted > 0 ? MEDAL_TONES[p.rank - 1] : null;
  const pct = topPercentile(p.rank, p.totalOperators);
  return (
    <div className="rounded-2xl border border-hairline bg-surface p-7 text-left shadow-[0_40px_90px_-50px_rgba(0,0,0,0.9)]">
      <div className="flex items-center gap-3.5">
        <Avatar
          user={{ id: "", username: p.username, hasAvatar: p.hasAvatar }}
          size={52}
          src={p.hasAvatar ? publicAvatarUrl(p.username) : undefined}
          className="border border-hairline-strong"
        />
        <div>
          <div className="flex items-center gap-2 text-17 font-semibold">
            {front ? (
              // Stop the pointer-down from reaching the card's drag handler:
              // otherwise the card captures the pointer and the click retargets to
              // the card (whose onClick is undefined) instead of this link, so the
              // handle would never navigate. Dragging still works from anywhere
              // else on the card.
              <Link
                to={`/u/${encodeURIComponent(p.username)}`}
                onPointerDown={(e) => e.stopPropagation()}
                className="cursor-pointer transition-colors hover:text-accent"
              >
                {p.username}
              </Link>
            ) : (
              p.username
            )}
            <span className="rounded-full border border-accent/35 bg-accent/10 px-2 py-0.5 font-mono text-10 text-accent">
              RANK #{p.rank}
            </span>
          </div>
          <div className="mt-0.5 font-mono text-xs text-slate-500">{joinedAgo(p.joinedAt)}</div>
        </div>
        <span className="ml-auto">{tone && <Medal tone={tone} />}</span>
      </div>

      <div className="mt-6">
        <ActivityHeatmap activity={p.activity} days={p.activityDays} />
      </div>

      <div className="mt-6 flex justify-center gap-9 border-t border-hairline pt-5 text-center">
        <CardStat v={String(p.longestStreakDays)} k="day streak" />
        <CardStat v={p.fastestSolveSeconds != null ? formatSolve(p.fastestSolveSeconds) : "-"} k="best time to root" />
        {pct && <CardStat v={pct} k="top percentile" />}
      </div>
    </div>
  );
}

// Skeleton while the showcase loads, shaped like the front card so the slide
// doesn't jump when the real deck arrives.
function DeckSkeleton() {
  return (
    <div className="mx-auto w-[540px] max-w-full" aria-hidden>
      <div className="rounded-2xl border border-hairline bg-surface p-7">
        <div className="flex items-center gap-3.5">
          <div className="h-[52px] w-[52px] animate-pulse rounded-full bg-slate-800" />
          <div className="space-y-2">
            <div className="h-4 w-40 animate-pulse rounded bg-slate-800" />
            <div className="h-3 w-52 animate-pulse rounded bg-slate-800" />
          </div>
        </div>
        <div className="mt-6 h-28 animate-pulse rounded bg-slate-800/50" />
        <div className="mt-6 h-10 animate-pulse rounded bg-slate-800/40" />
      </div>
    </div>
  );
}

// Shown only if the showcase can't load or the board is genuinely empty - the
// seed operators normally fill it, so this is the rare-failure path.
function DeckFallback() {
  return (
    <div className="mx-auto w-[540px] max-w-full rounded-2xl border border-hairline bg-surface p-10 text-center text-sm text-slate-400">
      The board is warming up. Take a look at the{" "}
      <Link to="/leaderboard" className="text-accent hover:underline">
        leaderboard
      </Link>
      .
    </div>
  );
}

// The lonely card, enriched: a shuffleable stack. Front card is crisp; the two
// behind are tilted, dimmed and grey. Click a back card (or a dot) to bring it
// forward - the transforms transition so it feels like flipping through a deck.
const SLOTS = [
  { transform: "rotate(0deg) translate(0,0) scale(1)", opacity: 1, z: 30, gray: 0 },
  { transform: "rotate(-6deg) translate(-9%,4%) scale(0.93)", opacity: 0.5, z: 20, gray: 0.4 },
  { transform: "rotate(6deg) translate(9%,4%) scale(0.93)", opacity: 0.5, z: 10, gray: 0.4 },
];

function StackedProfiles() {
  // The real top-3 operators (same live board the leaderboard serves), cached a
  // minute so revisiting the slide doesn't refetch.
  const { data, isPending, isError } = useQuery({
    queryKey: ["showcase-operators"],
    queryFn: getShowcaseOperators,
    refetchOnWindowFocus: false,
    staleTime: 60_000,
  });
  const cards = useMemo(() => (data ?? []).slice(0, 3), [data]);

  const [order, setOrder] = useState<number[]>([]);
  const [dragging, setDragging] = useState(false);
  const dragStart = useRef<number | null>(null);
  const deckRef = useRef<HTMLDivElement>(null);

  // Seed / reset the deck order once the operators arrive (or their count
  // changes). Until then order is empty and the skeleton shows.
  useEffect(() => {
    setOrder(cards.map((_, i) => i));
  }, [cards.length]);

  const bringFront = (ui: number) => setOrder((o) => [ui, ...o.filter((x) => x !== ui)]);
  const stepForward = () => setOrder((o) => (o.length ? [...o.slice(1), o[0]] : o)); // next
  const stepBack = () => setOrder((o) => (o.length ? [o[o.length - 1], ...o.slice(0, -1)] : o)); // previous

  // Left / right arrows step the carousel while the deck is on screen. Scoped to
  // visibility so the keys do nothing on the other slides (they don't scroll the
  // page anyway). Back cards are also clickable; the front card is draggable.
  useEffect(() => {
    const el = deckRef.current;
    if (!el || typeof IntersectionObserver === "undefined") return;
    let visible = false;
    const io = new IntersectionObserver(([e]) => (visible = e.isIntersecting), { threshold: 0.4 });
    io.observe(el);
    const onKey = (e: KeyboardEvent) => {
      if (!visible) return;
      if (e.key === "ArrowRight") {
        e.preventDefault();
        stepForward();
      } else if (e.key === "ArrowLeft") {
        e.preventDefault();
        stepBack();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => {
      io.disconnect();
      window.removeEventListener("keydown", onKey);
    };
  }, []);

  // Flick the front card: a press then a horizontal move past ~130px commits the
  // switch and lets the card-swap animation play out on its own - the card does
  // NOT follow the pointer. Drag left => next, drag right => previous. Pointer
  // capture keeps the gesture alive if you drift off the card; touch-pan-y below
  // still lets a vertical touch scroll the page.
  const THRESHOLD = 130;
  const onFrontDown = (e: ReactPointerEvent<HTMLDivElement>) => {
    dragStart.current = e.clientX;
    setDragging(true);
    try {
      e.currentTarget.setPointerCapture(e.pointerId);
    } catch {
      /* pointer already released / not capturable - the gesture still works */
    }
  };
  const onFrontMove = (e: ReactPointerEvent<HTMLDivElement>) => {
    if (dragStart.current == null) return;
    const dx = e.clientX - dragStart.current;
    if (Math.abs(dx) <= THRESHOLD) return;
    dragStart.current = null; // commit once, then ignore the rest of the gesture
    setDragging(false);
    (dx < 0 ? stepForward : stepBack)();
  };
  const onFrontUp = () => {
    dragStart.current = null;
    setDragging(false);
  };

  // order trails cards by one render (it's set in an effect), so wait for them to
  // agree before drawing the deck - otherwise indexOf/SLOTS would read undefined.
  if (isPending || order.length !== cards.length) return <DeckSkeleton />;
  if (isError || cards.length === 0) return <DeckFallback />;

  return (
    <div>
      <div ref={deckRef} className="mx-auto grid w-[540px] max-w-full">
        {cards.map((p, ui) => {
          const slot = order.indexOf(ui);
          const front = slot === 0;
          const s = SLOTS[slot] ?? SLOTS[SLOTS.length - 1];
          return (
            <div
              key={p.username}
              onClick={front ? undefined : () => bringFront(ui)}
              onPointerDown={front ? onFrontDown : undefined}
              onPointerMove={front ? onFrontMove : undefined}
              onPointerUp={front ? onFrontUp : undefined}
              onPointerCancel={front ? onFrontUp : undefined}
              aria-hidden={!front}
              className={`col-start-1 row-start-1 w-full touch-pan-y select-none ${
                front ? (dragging ? "cursor-grabbing" : "cursor-grab") : "cursor-pointer"
              }`}
              style={{
                transform: s.transform,
                opacity: s.opacity,
                zIndex: s.z,
                filter: `grayscale(${s.gray})`,
                transition:
                  "transform 0.45s cubic-bezier(0.4,0,0.2,1), opacity 0.45s ease, filter 0.45s ease",
              }}
            >
              <ProfileCard p={p} front={front} />
            </div>
          );
        })}
      </div>
      {cards.length > 1 && (
        <div className="mt-8 flex items-center justify-center gap-2.5">
          {cards.map((p, ui) => (
            <button
              key={p.username}
              onClick={() => bringFront(ui)}
              aria-label={`Show ${p.username}`}
              className={`h-2 w-2 rounded-full transition-colors ${order[0] === ui ? "bg-accent" : "bg-hairline-strong hover:bg-slate-600"}`}
            />
          ))}
        </div>
      )}
    </div>
  );
}

// Braille spinner for the fake-run animation.
function Spinner() {
  const frames = "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏";
  const [i, setI] = useState(0);
  useEffect(() => {
    if (reducedMotion()) return;
    const t = setInterval(() => setI((x) => (x + 1) % frames.length), 90);
    return () => clearInterval(t);
  }, []);
  return <span>{frames[i]}</span>;
}

// Editable fake terminal. The command is pre-filled but the visitor can rewrite
// it (capped at the original length). Whatever they run, Enter (or Run it) plays
// a few seconds of fake provisioning, then redirects to sign in - it looks and
// feels like starting a real session. No focus ring, per design.
function FakeTerminal({ onRun, authed }: { onRun: () => void; authed: boolean }) {
  const INITIAL = "start-session --lab ns-escape";
  const [value, setValue] = useState(INITIAL);
  const [caret, setCaret] = useState(INITIAL.length); // caret position, in characters
  const [running, setRunning] = useState(false);
  const [lines, setLines] = useState<string[]>([]);
  const inputRef = useRef<HTMLInputElement>(null);
  const timers = useRef<ReturnType<typeof setTimeout>[]>([]);

  useEffect(() => () => timers.current.forEach(clearTimeout), []);

  // Track the input's real caret so the custom block cursor sits exactly where
  // the text caret is (mono font => 1 char == 1ch of offset).
  const syncCaret = () => {
    const el = inputRef.current;
    if (el && el.selectionStart != null) setCaret(el.selectionStart);
  };

  const run = () => {
    if (running) return;
    setRunning(true);
    const seq = [
      "↳ allocating microVM",
      "↳ booting nested k3s cluster",
      "↳ applying lab manifests",
      authed
        ? "session ready - opening your labs…"
        : "auth required - sign in to claim this session…",
    ];
    seq.forEach((l, i) =>
      timers.current.push(setTimeout(() => setLines((p) => [...p, l]), 100 + i * 180)),
    );
    timers.current.push(setTimeout(onRun, 900));
  };

  return (
    <div>
      <div
        onClick={() => !running && inputRef.current?.focus()}
        className="cursor-text overflow-hidden rounded-xl border border-hairline bg-[#060b16] text-left"
      >
        <div className="flex h-9 items-center gap-2 border-b border-hairline bg-[#0d1526] px-3.5">
          <span className="h-2.5 w-2.5 rounded-full bg-slate-700" />
          <span className="h-2.5 w-2.5 rounded-full bg-slate-700" />
          <span className="h-2.5 w-2.5 rounded-full bg-slate-700" />
          <span className="ml-2 font-mono text-11 text-slate-500">k8labs</span>
          <span className="ml-auto font-mono text-10 text-slate-600">
            {running ? "running…" : "press ↵ to run"}
          </span>
        </div>
        <div className="p-4 font-mono text-13 leading-relaxed sm:p-5">
          {/* flex-wrap + basis-full let the command drop to its own full-width
              line on narrow screens (an <input> clips rather than wrapping, so on
              a phone the tail of the command was hidden); sm+ keeps it inline. */}
          <div className="flex flex-wrap">
            <span className="shrink-0 text-status-ok">you@k8labs</span>
            <span className="shrink-0 text-slate-300">:~$&nbsp;</span>
            {running ? (
              <span className="min-w-0 basis-full break-words text-slate-100 sm:basis-auto">{value || " "}</span>
            ) : (
              <span className="relative min-w-0 flex-1 basis-full sm:basis-0">
                <input
                  ref={inputRef}
                  value={value}
                  onChange={(e) => {
                    setValue(e.target.value);
                    setCaret(e.target.selectionStart ?? e.target.value.length);
                  }}
                  onSelect={syncCaret}
                  onClick={syncCaret}
                  onKeyUp={syncCaret}
                  onFocus={syncCaret}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault();
                      run();
                    }
                  }}
                  maxLength={INITIAL.length + 10}
                  spellCheck={false}
                  autoComplete="off"
                  aria-label="Terminal command - press Enter to run"
                  className="w-full border-0 bg-transparent p-0 text-slate-100 caret-transparent outline-none"
                />
                <span
                  aria-hidden
                  className="caret-blink pointer-events-none absolute top-1/2 h-[15px] w-[7px] -translate-y-1/2 bg-accent"
                  style={{ left: `${caret}ch` }}
                />
              </span>
            )}
          </div>
          {running ? (
            <div className="mt-3 space-y-1 text-slate-400">
              {lines.map((l, i) => (
                <div key={i}>{l}</div>
              ))}
              <div className="text-accent">
                <Spinner /> working…
              </div>
            </div>
          ) : (
            <div className="mt-2 text-slate-500">// one command from here. the rest is you.</div>
          )}
        </div>
      </div>
      <div className="mt-8 flex justify-center">
        <Cta onClick={run} label="Enter" />
      </div>
    </div>
  );
}

export default function Landing() {
  useSeo({ path: "/", type: "website" });
  const navigate = useNavigate();
  const { isAuthenticated, isLoading, serverDown, user, avatarVersion } = useAuth();
  const videoRef = useRef<HTMLVideoElement>(null);

  useEffect(() => {
    if (reducedMotion()) videoRef.current?.pause();
  }, []);

  // The body keeps its marketing copy for everyone; only the nav corner differs
  // by auth (sign-in prompt vs. a user icon into the app). Every CTA routes into
  // the app for a member and to sign-in for a visitor.
  const goPrimary = () => navigate(isAuthenticated ? "/labs" : "/login");

  return (
    <div className="h-screen snap-y snap-mandatory overflow-y-auto overflow-x-hidden bg-slate-950 text-slate-100 motion-reduce:snap-none">
      {/* Ambient escape-grid: one fixed viewport-sized field behind every slide,
          so the dots + roaming eBPF lens are continuous across the whole deck. */}
      <HeroEscapeBackground className="pointer-events-none fixed inset-0 h-full w-full" />
      <nav className="fixed inset-x-0 top-0 z-50 border-b border-hairline/40 bg-slate-950/70 backdrop-blur">
        <div className="landing-zoom relative flex h-20 items-center justify-between px-4">
          {/* Server-issue indicator, centered on the bar itself (absolute, so the
              logo/right widths don't push it off-center). Same treatment as
              PublicNav so the badge sits identically on every unauthed page. */}
          {serverDown && (
            <div className="pointer-events-none absolute inset-0 flex items-center justify-center">
              <div className="pointer-events-auto">
                <ServerIssueBadge />
              </div>
            </div>
          )}
          <button
            onClick={() => navigate("/")}
            className="flex items-center gap-3 rounded-lg text-lg font-semibold text-slate-100 transition-colors hover:text-white"
            aria-label="K8 Labs home"
          >
            <img src="/logo.svg" alt="" className="h-12 w-auto" />
            K8 Labs
          </button>
          {/* Right cluster: a browse affordance (Leaderboards, always public)
              paired with the auth control. Both controls share a 36px height so
              the pair sits level in the roomier h-20 bar. */}
          <div className="flex items-center gap-3">
            <button
              onClick={() => navigate("/leaderboard")}
              className="inline-flex h-9 items-center gap-1.5 rounded-lg px-3 text-13 text-slate-400 transition-colors hover:bg-white/5 hover:text-slate-100 focus:outline-none focus-visible:ring-2 focus-visible:ring-accent"
            >
              <Trophy size={15} className="text-slate-500" aria-hidden />
              Leaderboard
            </button>
            {isLoading ? (
              // Auth resolves after mount (getMe). Hold the corner with a pulsing
              // skeleton dot - matching the signed-in control's footprint - so a
              // member refreshing the page never sees a flash of the "Sign in"
              // button before the avatar lands. A filled pulse (not a bare ring)
              // so it reads as loading rather than an empty/broken circle.
              <div className="h-9 w-9 animate-pulse rounded-full bg-slate-800" aria-hidden />
            ) : isAuthenticated && user ? (
              <button
                onClick={goPrimary}
                aria-label="Go to your labs"
                title="Go to your labs"
                className="flex h-9 w-9 items-center justify-center rounded-full text-slate-300 ring-1 ring-hairline transition hover:ring-hairline-strong focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
              >
                <Avatar user={user} size={36} version={avatarVersion} />
              </button>
            ) : (
              <button
                onClick={goPrimary}
                className="inline-flex h-9 items-center rounded-lg bg-accent/10 px-3.5 text-13 font-medium text-accent ring-1 ring-inset ring-accent/25 transition-colors hover:bg-accent/15 hover:ring-accent/40 focus:outline-none focus-visible:ring-2 focus-visible:ring-accent"
              >
                Sign in
              </button>
            )}
          </div>
        </div>
      </nav>

      {/* SLIDE 1 - cinematic clip */}
      {/* The height-keyed --landing-zoom (index.css) shrinks the content so the
          whole hero fits one screen; with it fitting, justify-center supplies the
          top gap and the padding only needs to clear the fixed h-20 nav. Trimmed
          from pt-36/pb-24 (240px, unscaled by zoom - the reason short laptops
          still overflowed) to pt-24/pb-12 so the mock + CTA stay above the fold
          on a 768px-tall laptop. */}
      <section className="relative flex min-h-screen snap-start snap-always flex-col items-center justify-center px-6 pb-12 pt-24 text-center">
        <div
          aria-hidden
          className="pointer-events-none absolute left-1/2 top-1/2 h-[620px] w-[1100px] max-w-full -translate-x-1/2 -translate-y-1/2 rounded-full"
          style={{
            background: "radial-gradient(50% 50% at 50% 50%, rgba(106,166,242,0.18), transparent 70%)",
          }}
        />
        <div className="landing-zoom relative w-full max-w-[1330px]">
          <div className="font-mono text-[14.3px] uppercase tracking-[0.18em] text-accent/85">
            Hands-on Kubernetes attack &amp; defense
          </div>
          <div className="mt-6 overflow-hidden rounded-[16px] border border-hairline bg-surface shadow-[0_50px_100px_-35px_rgba(0,0,0,0.85),0_0_80px_-24px_rgba(106,166,242,0.28)]">
            <div className="flex h-9 items-center gap-2 border-b border-hairline bg-[#0d1526] px-3.5">
              <span className="h-2.5 w-2.5 rounded-full bg-slate-700" />
              <span className="h-2.5 w-2.5 rounded-full bg-slate-700" />
              <span className="h-2.5 w-2.5 rounded-full bg-slate-700" />
              <span className="ml-2 font-mono text-11 text-slate-500">eBPF Detection · session</span>
            </div>
            <video
              ref={videoRef}
              className="block w-full"
              autoPlay={!savePower()}
              muted
              loop
              playsInline
              preload={savePower() ? "none" : "metadata"}
              poster="/media/ebpf-lab-demo.poster.jpg"
            >
              <source src="/media/ebpf-lab-demo.webm" type="video/webm" />
              <source src="/media/ebpf-lab-demo.mp4" type="video/mp4" />
            </video>
          </div>
          <div className="mt-8 flex justify-center">
            <Cta onClick={goPrimary} label="Can you solve the entry lab?" />
          </div>
        </div>
      </section>

      {/* SLIDE 2 - typographic mission */}
      <section className="relative flex min-h-screen snap-start snap-always items-center border-t border-hairline px-6 py-24">
        <div className="landing-zoom mx-auto w-full max-w-6xl">
          <div className="h-0.5 w-14 bg-accent/70" />
          <p className="mt-6 font-mono text-13 text-accent/80">// why you&apos;re here</p>
          <h1 className="mt-5 text-[40px] font-bold leading-[0.98] tracking-tight sm:text-[56px] md:text-[76px]">
            Cloud security research
            <br />
            starts with a{" "}
            <span className="bg-gradient-to-r from-[#cfe3ff] to-accent bg-clip-text text-transparent">
              pod escape.
            </span>
          </h1>
          <div className="mt-10 grid gap-8 md:grid-cols-[1fr_auto] md:items-end">
            <p className="max-w-[34em] text-18 text-slate-400">
              We drop you into a real, disposable cluster with a root shell and a target to break -
              then a verifiable rank when you clear it. The kind that gets SRE and cloud-security
              interviews.
            </p>
            <div className="shrink-0">
              <Cta onClick={goPrimary} label="See if you can even get in" />
            </div>
          </div>
        </div>
      </section>

      {/* SLIDE 3 - stacked track-record cards */}
      <section className="relative flex min-h-screen snap-start snap-always items-center border-t border-hairline px-6 py-24">
        <div className="landing-zoom mx-auto w-full max-w-3xl text-center">
          <div className="font-mono text-[14.3px] uppercase tracking-[0.18em] text-accent/85">
            A profile that proves it
          </div>
          <h2 className="mt-4 text-[34px] font-semibold tracking-tight md:text-[46px]">
            Your track record, on the record.
          </h2>
          <div className="mt-12">
            <StackedProfiles />
          </div>
        </div>
      </section>

      {/* SLIDE 4 - editable fake terminal + footer */}
      <section className="relative flex min-h-screen snap-start snap-always flex-col items-center justify-center border-t border-hairline px-6 py-24 text-center">
        <div className="landing-zoom w-full max-w-xl">
          <FakeTerminal onRun={goPrimary} authed={isAuthenticated} />
        </div>
        <div className="absolute inset-x-0 bottom-0">
          <Footer />
        </div>
      </section>
    </div>
  );
}
