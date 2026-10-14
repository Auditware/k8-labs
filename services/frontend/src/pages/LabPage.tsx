import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useParams } from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  createSession,
  deleteSession,
  extendSession,
  getCourse,
  getCourseProgress,
  getModules,
  getSession,
  keepAliveSession,
  putCourseAnswer,
  resetCourseProgress,
  setActiveTab,
  setGuideStep,
  submitFlag,
  type Session,
  type StartSessionError,
} from "../lib/api";
import NestedTerminal from "../components/NestedTerminal";
import ProvisioningLoader from "../components/ProvisioningLoader";
import DefenderPane from "../components/DefenderPane";
import ResearchGuide from "../components/ResearchGuide";
import DriverPicker from "../components/DriverPicker";
import SessionTimer from "../components/SessionTimer";
import CoursePlayer, { type Step as CourseStepView } from "../components/course/CoursePlayer";
import ResizablePane from "../components/course/ResizablePane";
import { TerminalRunContext } from "../components/course/terminalRun";
import { useIsNarrow } from "../lib/useIsNarrow";
import ScopedTopologyMap from "../components/ScopedTopologyMap";
import AiAssistPanel from "../components/course/AiAssistPanel";
import { Button } from "../components/ui/Form";

// Where a running session's id is remembered so returning to this lab (reload,
// or navigating away and back) reconnects the terminal instead of stripping it
// back to "Start session". Keyed by slug: one active session per module.
// Cleared when the session is stopped, or when a reconnect attempt finds it
// already gone (reaped) - see the effect below.
//
// Deliberately just the id. The start time used to live here too and drive a
// client-side elapsed counter; both deadlines now come from the server on every
// session read (see api.ts's Session.expiresAt / idleExpiresAt), because a
// timer derived from browser storage drifts, is lost on another device, and can
// be edited by the student - none of which is acceptable for a number that says
// when their work disappears.
const sessionKey = (slug: string) => `k8labs:session:${slug}`;

type StoredSession = { id: string };

function readStored(slug: string): StoredSession | null {
  const raw = localStorage.getItem(sessionKey(slug));
  if (!raw) return null;
  try {
    const parsed = JSON.parse(raw) as Partial<StoredSession>;
    return typeof parsed?.id === "string" ? { id: parsed.id } : null;
  } catch {
    localStorage.removeItem(sessionKey(slug));
    return null;
  }
}

// A quiet way to reach us, trailing an error line. Same size as the text it
// follows, muted until hovered. Opens a Telegram chat in a new tab; rel keeps
// the lab URL out of the referrer. This is a plain navigation, not a fetch, so
// it isn't subject to the SPA's connect-src CSP - no allowlist change needed.
function TelegramLink() {
  return (
    <a
      href="https://t.me/forefy_t"
      target="_blank"
      rel="noreferrer"
      aria-label="Message us on Telegram"
      className="ml-1 inline-flex align-middle text-slate-500 transition-colors hover:text-blue-400"
    >
      <svg viewBox="0 0 24 24" width="1em" height="1em" fill="currentColor" aria-hidden="true">
        <path d="M9.78 18.65l.28-4.23 7.68-6.92c.34-.31-.07-.46-.52-.19L7.74 13.3 3.64 12c-.88-.25-.89-.86.2-1.3l15.97-6.16c.73-.33 1.43.18 1.15 1.3l-2.72 12.81c-.19.91-.74 1.13-1.5.71L12.6 16.3l-1.99 1.93c-.23.23-.42.42-.83.42z" />
      </svg>
    </a>
  );
}

// The student-facing sentence for a lab that's no longer running. reason comes
// from the backend tombstone (internal/labs EndedSession); the default covers
// "gone" - the tombstone has aged out, so all we can still say is that it ended.
function endedMessage(session: Session | null | undefined): string {
  const idleMin = session?.idleSeconds
    ? Math.max(1, Math.round(session.idleSeconds / 60))
    : null;
  switch (session?.reason) {
    case "idle":
      return idleMin
        ? `The lab was auto-closed after ${idleMin} min of inactivity.`
        : "The lab was auto-closed after a period of inactivity.";
    case "expired":
      return "The lab reached its time limit and closed.";
    case "stopped":
      return "This lab was stopped.";
    default:
      return "This lab session ended.";
  }
}

// The failed-to-start line. node_capacity is a "not you, our cluster is full"
// case, so we soften it and offer a way to reach us; everything else (quota,
// especially) already carries an actionable sentence from the backend, so we
// show that as-is rather than burying it.
function StartError({ err }: { err: StartSessionError | null }) {
  if (!err) return null;
  if (err.reason === "node_capacity") {
    return (
      <p className="max-w-sm text-xs text-red-400">
        Our cluster capacity was reached, please send us a message if this continues
        <TelegramLink />
      </p>
    );
  }
  // A server error (or an unreachable backend surfaced as a 5xx) is on our side,
  // so carry the same "reach us if it persists" affordance as capacity - offer it
  // alongside the friendly message set in createSession rather than dead-ending.
  if (err.reason === "server_error") {
    return (
      <p className="max-w-sm text-xs text-red-400">
        {err.message}
        <TelegramLink />
      </p>
    );
  }
  return <p className="max-w-sm text-xs text-red-400">{err.message}</p>;
}

export default function LabPage() {
  const { slug } = useParams<{ slug: string }>();
  const narrow = useIsNarrow();
  const [sessionId, setSessionId] = useState<string | null>(null);
  // Research-driver ids the user picked to mount into the next session (research
  // labs only). Passed into createSession; ignored by every other lab.
  const [selectedDrivers, setSelectedDrivers] = useState<string[]>([]);
  // On mobile the course is a tab too, so "guide" is a valid active tab and the
  // default (you read before you get a shell); on desktop the guide is always
  // visible in its own pane, so the right pane defaults to the terminal.
  type RightTab = "guide" | "terminal" | "topology" | "defender" | "ai";
  const [rightTab, setRightTab] = useState<RightTab>(
    () => (typeof window !== "undefined" && window.innerWidth < 768 ? "guide" : "terminal"),
  );
  // Which right-pane tabs have been opened. A tab's pane (terminal/defender) holds
  // a live websocket into the session; once opened we keep it MOUNTED and just
  // hide it on switch, so flipping tabs doesn't tear down and rebuild the
  // connection (a slow reconnect: a fresh exec shell, or waitForDefenderPod's
  // ~20s poll). Terminal is open by default; others mount lazily on first visit.
  const [openedTabs, setOpenedTabs] = useState<Set<string>>(() => new Set(["terminal"]));
  const openTab = (t: RightTab) => {
    setRightTab(t);
    setOpenedTabs((prev) => (prev.has(t) ? prev : new Set(prev).add(t)));
  };
  const queryClient = useQueryClient();
  const [fullscreen, setFullscreen] = useState(false);

  const { data: modules, isPending: modulesPending } = useQuery({
    queryKey: ["modules"],
    queryFn: getModules,
  });
  const module = modules?.find((m) => m.slug === slug);

  // Tell the backend which tab is open, so the AI's /context knows where the
  // student's attention is (terminal/topology/ai/guide/defender). Best-effort.
  useEffect(() => {
    if (sessionId) setActiveTab(sessionId, rightTab);
  }, [sessionId, rightTab]);

  // The student's position in the guided course, reported by CoursePlayer.
  // Relayed to the backend so the AI tutor paces with them. Keyed on both, so a
  // session that starts AFTER the student has paged ahead still gets the current
  // step (the course-page effect alone wouldn't re-fire on sessionId change).
  const [guidePos, setGuidePos] = useState<{ index: number; total: number } | null>(null);
  useEffect(() => {
    if (sessionId && guidePos) setGuideStep(sessionId, guidePos.index, guidePos.total);
  }, [sessionId, guidePos]);

  const start = useMutation({
    mutationFn: () => createSession(slug!, selectedDrivers),
    onSuccess: (session) => {
      setSessionId(session.id);
      // Starting from a non-terminal tab (topology/ai/guide) jumps to the
      // terminal, where the provisioning animation lives - so the wait has a
      // home instead of stranding the student on a tab that can't show progress.
      openTab("terminal");
      localStorage.setItem(sessionKey(slug!), JSON.stringify({ id: session.id }));
      // A new session is a new node on the admin map. Mark it stale so the
      // admin dashboard (same client, e.g. an admin running a lab) repaints on
      // its next mount/tick instead of waiting out the poll.
      queryClient.invalidateQueries({ queryKey: ["admin-topology"] });
    },
  });

  const stop = useMutation({
    mutationFn: () => deleteSession(sessionId!),
    onSuccess: () => {
      setSessionId(null);
      setFullscreen(false);
      if (slug) localStorage.removeItem(sessionKey(slug));
      // The session is gone from the backend synchronously (labs.Manager.Delete
      // drops it immediately), so invalidate the admin map: navigating to the
      // dashboard right after closing a lab shows it dropped off at once,
      // rather than serving the cached map that still lists the dead session.
      queryClient.invalidateQueries({ queryKey: ["admin-topology"] });
    },
  });

  // Extend a running research session's hard cap by 2h. On success we write the
  // returned (freshly-dated) session straight into the query cache so the
  // countdown jumps immediately, without waiting for the 15s status poll.
  const extend = useMutation({
    mutationFn: () => extendSession(sessionId!),
    onSuccess: (updated) => {
      queryClient.setQueryData(["session", sessionId], updated);
    },
  });

  // "Stop that lab and start this one." When starting is refused because the user
  // already has a different lab running (the backend names it on the 409), this
  // stops the offending session and immediately starts this module - turning a
  // dead-end error into one click. Delete is synchronous on the backend, so the
  // create that follows sees the freed slot.
  const swap = useMutation({
    mutationFn: async ({ conflictId }: { conflictId: string; conflictModule?: string }) => {
      // Tolerate the other lab already being gone (reaped, or stopped elsewhere) -
      // the create below is the real gate and 409s again if a session genuinely
      // still holds the slot.
      await deleteSession(conflictId).catch(() => {});
      return createSession(slug!, selectedDrivers);
    },
    onSuccess: (session, { conflictModule }) => {
      setSessionId(session.id);
      openTab("terminal"); // land on the loader (see start)
      localStorage.setItem(sessionKey(slug!), JSON.stringify({ id: session.id }));
      // The lab we just stopped is a different slug; drop its stored session so
      // its own page won't try to reconnect to a now-dead cluster.
      if (conflictModule) localStorage.removeItem(sessionKey(conflictModule));
      queryClient.invalidateQueries({ queryKey: ["admin-topology"] });
    },
  });

  // "Try again" from the failed/ended state. A session that errored or was reaped
  // is already dead, so recovery is one click, not stop-then-start: clean up the
  // husk (tolerating it already being gone) and start a fresh one.
  const retry = useMutation({
    mutationFn: async () => {
      if (sessionId) await deleteSession(sessionId).catch(() => {});
      return createSession(slug!, selectedDrivers);
    },
    onSuccess: (session) => {
      setSessionId(session.id);
      openTab("terminal"); // land on the loader (see start)
      if (slug) localStorage.setItem(sessionKey(slug), JSON.stringify({ id: session.id }));
      queryClient.invalidateQueries({ queryKey: ["admin-topology"] });
    },
  });

  // Poll the session while it provisions. Nested modules build a whole
  // throwaway cluster in the background (see internal/labs' provisionNested),
  // so a freshly-created session starts as "starting" and only becomes
  // "running" once its foothold is up - the terminal must wait for that.
  // Legacy modules come back "running" immediately, so this just resolves once.
  const { data: session } = useQuery({
    queryKey: ["session", sessionId],
    queryFn: () => getSession(sessionId!),
    enabled: !!sessionId,
    refetchInterval: (query) => {
      const s = query.state.data;
      // Stop only once there is nothing left to learn: a genuine start failure
      // (error), a reaped session's tombstone (ended), or a session fully gone
      // (404 -> null, tombstone aged out). These are terminal - the record never
      // changes again.
      if (s === null) return false;
      if (s && (s.status === "error" || s.status === "ended")) return false;
      // A "running" session is NOT terminal for polling: idleExpiresAt slides
      // forward on every keystroke server-side (see labs/terminal.go's Activity),
      // and the reaper can still end the lab (idle or hard cap) out from under a
      // client that has stopped asking. Keep a slow poll so the idle countdown
      // stays truthful while the student types - otherwise the SessionTimer badge
      // counts a frozen deadline down to zero even during active use - and so a
      // reap flips the pane to its real end-reason promptly rather than only when
      // the terminal socket happens to drop.
      if (s && s.status === "running") return 15000;
      // 1s while provisioning: the loader's progress bar anchors to the session's
      // reported phase (see ProvisioningLoader), so a faster poll surfaces gate
      // transitions sooner and makes the bar move in step with the real boot -
      // and reveals the terminal a touch sooner once it flips to "running". One
      // small GET/sec against an in-memory lookup is cheap.
      return 1000;
    },
    // Keep polling even when the tab is backgrounded. Without this, React Query
    // pauses refetchInterval whenever the page loses focus, so a student who
    // starts a lab and switches away (or whose pane simply isn't the focused
    // surface) can have the provisioning poll stall after its first tick - the
    // session reaches "running" server-side but the boot loader never clears
    // until a manual reload refetches. Provisioning is a bounded ~40s window and
    // the poll is one cheap GET/sec, so running it in the background is worth the
    // loader always advancing to the live terminal on its own.
    refetchIntervalInBackground: true,
  });
  // Until the first poll returns, assume "starting" (a just-created session).
  const status = sessionId ? (session === null ? "gone" : (session?.status ?? "starting")) : null;

  // Presence keepalive. The idle reaper only counts terminal INPUT as activity
  // (see labs/terminal.go), so a researcher WATCHING a long fuzzer or build - all
  // output, no typing - would be idle-reaped mid-job. A lab tab left open in the
  // foreground IS presence: the classic research pattern is parking the session
  // in a visible tab while the job (or the AI driving it) runs untouched for an
  // hour, and requiring recent mouse/key interaction on top of visibility reaped
  // exactly those sessions mid-work. So while this lab tab is visible, ping the
  // server to reset the idle clock - no interaction required. Gated on
  // document.visibilityState so a hidden or backgrounded tab stops pinging and a
  // truly closed/abandoned session still reaps at its idle timeout. Fires
  // immediately on load and on the tab regaining visibility, then every 60s -
  // well under the shortest idle window - and only while the session is running.
  // Slides the idle deadline only, never the hard cap.
  useEffect(() => {
    if (!sessionId || session?.status !== "running") return;
    const ping = () => {
      if (document.visibilityState === "visible") void keepAliveSession(sessionId);
    };
    const onVisible = () => {
      if (document.visibilityState === "visible") ping();
    };
    document.addEventListener("visibilitychange", onVisible);
    ping();
    const iv = window.setInterval(ping, 60_000);
    return () => {
      window.clearInterval(iv);
      document.removeEventListener("visibilitychange", onVisible);
    };
  }, [sessionId, session?.status]);
  // In the ended/error pane below, only "error" is a genuine provisioning failure
  // (broke on our side); "ended" and "gone" are normal lifecycle ends - you
  // stopped the lab, it idled out, or it hit its time limit. The pane styles the
  // two apart so a normal end isn't painted in the same alarm-red as a real
  // failure (which read as if something had broken when the student ended it on
  // purpose). Calm + a low-emphasis "Start again" for ends; red + a primary "Try
  // again" + a way to reach us for the failure.
  const provisionFailed = status === "error";

  // Reconnect to a still-running session on mount / slug change, instead of
  // always dropping back to "Start session". A session outlives the page:
  // it's the backend's idle/max-age reaper (internal/labs/session.go) that
  // stops it, not the browser closing - so after a reload or a round trip
  // to another page, its terminal should come straight back. We don't stop
  // the session on unload for exactly this reason: killing it on every
  // reload would make reconnect impossible, and the reaper already bounds
  // the cost of a truly-abandoned session. First clear any state from the
  // previous slug, then adopt the stored session only if the backend still
  // has it (a reaped one 404s -> clear the stale local record).
  useEffect(() => {
    if (!slug) return;
    setSessionId(null);
    setFullscreen(false);

    const stored = readStored(slug);
    if (!stored) return;

    let cancelled = false;
    getSession(stored.id)
      .then((session) => {
        if (cancelled) return;
        if (session && (session.status === "running" || session.status === "starting")) {
          setSessionId(session.id);
        } else {
          localStorage.removeItem(sessionKey(slug));
        }
      })
      .catch(() => {
        // Transient lookup failure (e.g. backend momentarily unreachable) -
        // leave the stored record in place to retry on the next mount rather
        // than discarding a possibly-still-live session.
      });
    return () => {
      cancelled = true;
    };
  }, [slug]);


  // The lesson and the student's place in it. Both are entitlement-gated
  // server-side, so an ungranted module 403s here rather than rendering.
  const { data: course } = useQuery({
    queryKey: ["course", slug],
    queryFn: () => getCourse(slug!),
    enabled: !!slug,
    staleTime: Infinity, // content is embedded in the backend build - it cannot change mid-session
  });
  const { data: progress } = useQuery({
    queryKey: ["course-progress", slug],
    queryFn: () => getCourseProgress(slug!),
    enabled: !!slug,
  });

  // Answers are written through to the server and the server's progress replaces
  // ours. Quizzes are formative - they never mark the lab complete; only the
  // flag does (below).
  const answer = useMutation({
    mutationFn: ({ stepSlug, option }: { stepSlug: string; option: number }) =>
      putCourseAnswer(slug!, stepSlug, option),
    onSuccess: (p) => queryClient.setQueryData(["course-progress", slug], p),
  });

  // The flag is the completion gate. On a correct submit the server has stamped
  // completion, so flip our cached `complete` (and clear the leaderboard cache,
  // which now shows a new solve). The CoursePlayer form reads `correct` from the
  // returned value to drive its own state, so return it through.
  const flag = useMutation({
    mutationFn: (value: string) => submitFlag(slug!, value),
    onSuccess: (res) => {
      if (res.correct) {
        queryClient.setQueryData(["course-progress", slug], (prev: unknown) =>
          prev ? { ...(prev as object), complete: true } : prev,
        );
        queryClient.invalidateQueries({ queryKey: ["leaderboard"] });
        // A solve can unlock the next progression tier, so refetch the catalog:
        // newly entitled labs appear in the sidebar without a manual reload.
        queryClient.invalidateQueries({ queryKey: ["modules"] });
      }
    },
  });

  // Restart clears this lab's progress server-side; refetch so answers empty out
  // and `complete` goes false, which re-enables the quizzes and resets the form.
  const reset = useMutation({
    mutationFn: () => resetCourseProgress(slug!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["course-progress", slug] });
      queryClient.invalidateQueries({ queryKey: ["leaderboard"] });
    },
  });

  const steps = (course?.steps ?? []) as CourseStepView[];

  // One way to start the lab, reused by the course's "Start →" button and the
  // right-pane placeholder. Idempotent-ish: never fires a second Create while
  // one is in flight or a session already exists.
  const startLab = () => {
    if (!sessionId && !start.isPending) start.mutate();
  };

  // --- Runnable-command bridge: the course's command chips (in the left pane)
  // paste into the terminal (in the right pane). The terminal registers a raw
  // "send to the shell" fn when its socket opens; Run makes sure a session and
  // the terminal tab exist, then sends the command, queueing it until the shell
  // connects if the lab is still provisioning. Kept stable (refs, not deps) so
  // the context value never churns and re-triggers consumers each render. ---
  const termSendRef = useRef<((data: string) => void) | null>(null);
  // Each queued command carries the resolver of the promise runCommand handed
  // back, so the Run button that queued it learns the moment it's actually sent.
  const pendingCmdsRef = useRef<Array<{ cmd: string; resolve: () => void }>>([]);
  const startLabRef = useRef(startLab);
  startLabRef.current = startLab;
  const flushPending = useCallback(() => {
    if (!termSendRef.current || pendingCmdsRef.current.length === 0) return;
    const queued = pendingCmdsRef.current;
    pendingCmdsRef.current = [];
    // Stagger a touch so the freshly-connected shell has drawn its prompt before
    // the first command lands, and several queued commands don't collide.
    queued.forEach((item, i) =>
      window.setTimeout(() => {
        termSendRef.current?.(item.cmd + "\r");
        item.resolve();
      }, 300 + i * 120),
    );
  }, []);
  const registerTerminal = useCallback(
    (send: (data: string) => void) => {
      termSendRef.current = send;
      flushPending();
      return () => {
        if (termSendRef.current === send) termSendRef.current = null;
      };
    },
    [flushPending],
  );
  const runCommand = useCallback((cmd: string): Promise<void> => {
    setRightTab("terminal");
    setOpenedTabs((prev) => (prev.has("terminal") ? prev : new Set(prev).add("terminal")));
    startLabRef.current();
    // Connected already: send now and the wait is over immediately. Otherwise
    // queue it and resolve later, when flushPending writes it on socket-open.
    if (termSendRef.current) {
      termSendRef.current(cmd + "\r");
      return Promise.resolve();
    }
    return new Promise<void>((resolve) => {
      pendingCmdsRef.current.push({ cmd, resolve });
    });
  }, []);
  const terminalRun = useMemo(
    () => ({ runCommand, registerTerminal }),
    [runCommand, registerTerminal],
  );

  // If the session never reaches a shell (it errored, or was reaped), settle any
  // still-queued commands so their Run buttons stop showing a pending state -
  // the failure itself is already spelled out in the terminal pane. Resolve
  // (not reject): "we're done waiting", not "this specific command failed".
  useEffect(() => {
    if (status !== "error" && status !== "gone") return;
    const queued = pendingCmdsRef.current;
    pendingCmdsRef.current = [];
    queued.forEach((item) => item.resolve());
  }, [status]);

  // The right pane's not-yet-started state: a real Start button where the
  // "start the session to…" sentence used to be, so the affordance is where the
  // student is looking when they pick a tab.
  //
  // When starting is refused because another lab is already running, the backend
  // names it, so instead of dead-ending on the error we offer to stop that lab
  // and start this one in a single click.
  const startErr = start.error as StartSessionError | null;
  // If the refusal names THIS lab (conflictModule === slug), the student isn't
  // switching labs - they already have this very lab running but we couldn't
  // reconnect them from localStorage (a slug rename, another browser, cleared
  // storage, or a session started out-of-band). The server just told us the id,
  // so adopt it and show the live terminal instead of the nonsensical "stop this
  // lab to start this lab" swap. readStored is best-effort; this 409 is the
  // authoritative "you have a session here" signal it can't get any other way.
  const selfConflict =
    start.isError &&
    startErr?.reason === "user_session_limit" &&
    !!startErr.conflictSessionId &&
    startErr.conflictModule === slug;
  useEffect(() => {
    if (!selfConflict || !slug || !startErr?.conflictSessionId) return;
    setSessionId(startErr.conflictSessionId);
    localStorage.setItem(sessionKey(slug), JSON.stringify({ id: startErr.conflictSessionId }));
    start.reset(); // clear the error so the placeholder yields to the terminal
  }, [selfConflict, slug, startErr, start]);
  const conflict =
    start.isError &&
    startErr?.reason === "user_session_limit" &&
    startErr.conflictSessionId &&
    startErr.conflictModule !== slug // a self-conflict reconnects above, never swaps
      ? {
          id: startErr.conflictSessionId,
          module: startErr.conflictModule,
          name:
            modules?.find((m) => m.slug === startErr.conflictModule)?.displayName ??
            startErr.conflictModule ??
            "your other lab",
        }
      : null;
  const startPlaceholder = (
    <div className="flex h-full flex-col items-center justify-center gap-3 px-6 text-center">
      {conflict ? (
        // A session-limit refusal replaces "Start session" outright, rather than
        // sitting beside it: with a lab already running, plain "Start" would just
        // hit the same limit, so the only useful action is the swap.
        <>
          <Button
            onClick={() => swap.mutate({ conflictId: conflict.id, conflictModule: conflict.module })}
            disabled={swap.isPending}
            className="px-5"
          >
            {swap.isPending ? "Switching…" : `Stop "${conflict.name}" and start this one`}
          </Button>
          <p className="max-w-xs text-xs text-slate-600">
            You already have a lab running. Starting this one stops it first.
          </p>
          {/* The swap is a tolerant delete + a createSession, so its failure IS a
              start failure - surface the same specific, humane reason (unreachable
              server, a 5xx, a fresh quota/conflict) rather than a flat "Couldn't
              switch labs" that hides what actually went wrong. */}
          {swap.isError && <StartError err={swap.error as StartSessionError | null} />}
        </>
      ) : (
        <>
          <Button
            onClick={startLab}
            disabled={start.isPending || swap.isPending || !slug}
            className="px-5"
          >
            {start.isPending ? "Starting session…" : "Start session"}
          </Button>
          <p className="max-w-xs text-xs text-slate-600">
            Boots a disposable cluster and starts your clock.
          </p>
          {start.isError && <StartError err={startErr} />}
        </>
      )}
    </div>
  );

  // The course pane (lab-title header + CoursePlayer). Rendered once: as the
  // split's left/top pane on desktop, or as the "guide" tab's content on mobile.
  // Kept in a variable so it mounts in exactly one place per layout - never twice
  // (double course state), and stays mounted across tab switches on mobile.
  const courseSection = (
    <>
      <div className="flex h-14 shrink-0 items-center justify-between border-b border-slate-800 px-4 md:h-20">
        <div className="min-w-0">
          {/* h2 for the lab (the app's own h1 is the brand), h3 for each step
              inside the player - a real heading hierarchy, not styled text. */}
          <h2 className="truncate text-13 font-medium text-slate-200">
            {module?.displayName ?? slug}
          </h2>
        </div>
        <div className="flex shrink-0 items-center gap-3 pl-3">
          {progress?.complete && (
            <span
              className="rounded-full bg-emerald-500/10 px-2 py-0.5 font-mono text-10 uppercase tracking-wider text-emerald-400 ring-1 ring-inset ring-emerald-500/30"
              title="You captured this lab's flag."
            >
              ✓ solved
            </span>
          )}
          {progress && progress.quizCount > 0 && (
            <span
              className={`font-mono text-11 tabular-nums ${
                progress.answered === progress.quizCount ? "text-emerald-400" : "text-slate-500"
              }`}
              title="Self-checks answered. These are formative - the flag is what completes the lab."
            >
              {progress.answered}/{progress.quizCount}
            </span>
          )}
          {sessionId && (
            <span className="flex items-center gap-2">
              <SessionTimer
                expiresAt={session?.expiresAt}
                idleExpiresAt={session?.idleExpiresAt}
                // Only research labs are extendable server-side; offer the button
                // only there, and only while the session is actually running.
                onExtend={
                  module?.track === "research" && session?.status === "running"
                    ? () => extend.mutate()
                    : undefined
                }
                extending={extend.isPending}
              />
              {extend.isError && (
                <span
                  className="font-mono text-10 text-amber-400"
                  title={(extend.error as Error)?.message}
                >
                  {(extend.error as Error)?.message ?? "extend failed"}
                </span>
              )}
            </span>
          )}
        </div>
      </div>

      <div className="min-h-0 flex-1">
        {steps.length > 0 ? (
          <CoursePlayer
            // Key per lab so switching modules remounts the player: its
            // remembered screen is per-lab, and a shared instance would carry
            // one lab's page index into the next.
            key={slug}
            steps={steps}
            answers={progress?.answers ?? {}}
            onAnswer={(stepSlug, option) => answer.mutate({ stepSlug, option })}
            moduleName={module?.displayName ?? slug ?? "this lab"}
            courseKey={slug}
            complete={progress?.complete ?? false}
            // Lets the player defer its opening placement until completion is the
            // loaded truth, so a solved lab opens on the capstone even when the
            // progress fetch lands after the course content.
            progressReady={progress !== undefined}
            onSubmitFlag={(value) => flag.mutateAsync(value)}
            onReset={() => reset.mutate()}
            // Leaving the opening screen (the "Start →" button) also boots the
            // lab, so there's a single start affordance, not a separate button.
            onStart={startLab}
            // Track the student's course position so we can tell the AI tutor
            // where they are (relayed to the backend once a session exists).
            onStepChange={(index, total) => setGuidePos({ index, total })}
          />
        ) : modulesPending ? (
          // The catalog hasn't resolved yet, so `module` is undefined and we
          // don't know this lab's track. Show a neutral loading state rather
          // than falling through to the non-research "About" panel below -
          // otherwise a research lab's driver picker/upload is hidden behind a
          // plain-lab layout until /modules lands, briefly on a warm fetch and
          // persistently on a slow or failed one. That flash is exactly what
          // made the driver-research-dev lab look like it had no upload.
          <div className="flex h-full items-center justify-center px-6 py-8">
            <div className="text-xs uppercase tracking-wider text-slate-600">Loading lab…</div>
          </div>
        ) : module?.track === "research" ? (
          // Research labs have no guided course by design (discovery, not
          // capture-the-flag). Instead of an apologetic empty line, show a
          // one-page visual guide: the exploit-capability flow on top, concise
          // text below. The -hard lab differs only in one caption.
          <ResearchGuide
            description={module?.description}
            hardened={module?.slug === "driver-research-hard"}
            picker={
              <DriverPicker
                selected={selectedDrivers}
                onChange={setSelectedDrivers}
                // Drivers are mounted at provision, so selection is only editable
                // before the session exists; lock it to a summary once running.
                disabled={!!sessionId}
                // Lets the picker flag a .ko that cannot insmod into this lab's
                // guest kernel before the session is provisioned, instead of the
                // researcher finding out from an opaque insmod error mid-session.
                expectedVermagic={module?.expectedVermagic}
              />
            }
          />
        ) : (
          // Any other course-less lab: a simple About panel (the course-pane
          // header already shows the lab name, so no title here).
          <div className="h-full overflow-y-auto px-6 py-8">
            <div className="mx-auto max-w-md">
              <div className="text-xs font-semibold uppercase tracking-wider text-indigo-400">
                About this lab
              </div>
              {module?.description && (
                <p className="mt-3 text-sm leading-relaxed text-slate-400">{module.description}</p>
              )}
              {module?.hint && (
                <div className="mt-4 rounded-lg border border-slate-800 bg-slate-900/60 p-3 text-left">
                  <div className="text-xs font-semibold uppercase tracking-wider text-slate-500">
                    Start here
                  </div>
                  <p className="mt-1 text-sm leading-relaxed text-slate-400">{module.hint}</p>
                </div>
              )}
              <p className="mt-4 text-xs leading-relaxed text-slate-600">
                No guided course for this lab yet - the terminal on the right is fully live.
              </p>
            </div>
          </div>
        )}
      </div>
    </>
  );

  return (
    <TerminalRunContext.Provider value={terminalRun}>
    <div className="h-full">
      <ResizablePane
        collapsed={narrow || fullscreen}
        right={
          <>
            {/* h-20 matches the sidebar's header block (SideNav's own h-20), so
                the divider under it continues the sidebar's in one straight line
                across the whole window instead of stepping down 32px at the
                sidebar's edge. Same for the course header below. */}
            <div className="flex h-14 shrink-0 items-center gap-1 border-b border-slate-800 bg-slate-900 px-3 md:h-20">
              {/* Tabs are conditional. Topology maps a K8s attack surface, which
                  the research labs don't have (the target is the guest kernel, in
                  one pod), so it's dropped there as noise - the same reason those
                  labs skip the flag capstone. The Defender tab only exists for
                  modules that ship an in-guest sensor (Module.HasDefender - the
                  eBPF and kernel-research labs). On mobile the course folds in
                  here as the "guide" tab so the one visible pane gets the whole
                  screen (see useIsNarrow). */}
              {([
                ...(narrow ? ["guide"] : []),
                "terminal",
                // Only once the module is resolved do we know its track; until
                // then show neither the topology tab (which would wrongly appear
                // on a still-loading research lab) nor force it off a real
                // non-research lab once known.
                ...(module !== undefined && module.track !== "research" ? ["topology"] : []),
                ...(module?.defender ? ["defender"] : []),
                "ai",
              ] as Array<
                RightTab
              >).map((t) => (
                <button
                  key={t}
                  onClick={() => openTab(t)}
                  className={`flex items-center gap-2 rounded-md px-3 py-1.5 text-13 capitalize transition-colors ${
                    rightTab === t
                      ? "bg-slate-800 text-slate-100"
                      : "text-slate-500 hover:text-slate-300"
                  }`}
                >
                  {t === "terminal" && (
                    // Status dot reflects the session: fading gray<->green while
                    // provisioning, solid green when running, red on error, gray
                    // when stopped/ended or no session.
                    <span
                      className={`h-1.5 w-1.5 rounded-full ${
                        status === "running"
                          ? "bg-emerald-500"
                          : status === "starting"
                            ? "dot-progress"
                            : status === "error"
                              ? "bg-red-500"
                              : "bg-slate-600"
                      }`}
                    />
                  )}
                  {/* `capitalize` turns "terminal" into "Terminal" but mangles
                      acronyms ("ai" -> "Ai", "eBPF" -> "EBPF"), so render those
                      literally. The sensor tab is "Defender" (a blue-team view)
                      on the CTF labs but plain "eBPF" instrumentation on the
                      research labs - name it for what it is in each context. */}
                  {t === "ai" ? (
                    "AI"
                  ) : t === "defender" && module?.track === "research" ? (
                    <span className="normal-case">eBPF</span>
                  ) : (
                    t
                  )}
                </button>
              ))}
              <div className="ml-auto flex items-center gap-2.5">
                {sessionId && (
                  <>
                    {/* Pop the ACTIVE pane into its own window (see PoppedPane),
                        so the live view and the course don't have to share one
                        screen - the fix for constantly resizing the two on a
                        small laptop. It mirrors whichever tab is showing: the
                        terminal, or the eBPF sensor on the research/eBPF labs -
                        the same way the enlarge toggle expands whichever pane is
                        active. It rebinds to this same running session, so it's
                        only offered once there's a live pane to attach to. */}
                    {status === "running" && (
                      <button
                        onClick={() => {
                          const pane = rightTab === "defender" ? "defender" : "terminal";
                          window.open(
                            `/labs/${slug}/${pane}`,
                            `k8labs-${pane}-${slug}`,
                            "popup,width=920,height=640",
                          );
                        }}
                        title={
                          rightTab === "defender"
                            ? "Open the eBPF sensor in a separate window"
                            : "Open the terminal in a separate window"
                        }
                        className="rounded p-1.5 text-slate-400 hover:bg-slate-800 hover:text-slate-200"
                      >
                        ⧉
                      </button>
                    )}
                    {/* Hide-the-course toggle is a split-view affordance; on mobile
                        the guide is its own tab, so there's nothing to hide. */}
                    {!narrow && (
                      <button
                        onClick={() => setFullscreen((f) => !f)}
                        title={fullscreen ? "Show the course" : "Hide the course"}
                        className="rounded p-1.5 text-slate-400 hover:bg-slate-800 hover:text-slate-200"
                      >
                        {fullscreen ? "⤡" : "⤢"}
                      </button>
                    )}
                    <Button
                      variant="danger"
                      size="sm"
                      onClick={() => stop.mutate()}
                      disabled={stop.isPending}
                      className="bg-red-500/10 hover:bg-red-500/20"
                    >
                      {stop.isPending ? "Stopping…" : "Stop lab"}
                    </Button>
                  </>
                )}
              </div>
            </div>

            <div className="min-h-0 flex-1">
              {/* Guide as a tab (mobile only). Kept mounted - hidden, not
                  unmounted - so the reader's step position survives flipping to
                  the terminal and back. On desktop the guide is in its own split
                  pane, so this never renders. */}
              {narrow && (
                <div className={rightTab === "guide" ? "flex h-full flex-col" : "hidden"}>
                  {courseSection}
                </div>
              )}
              {/* Non-pane content: topology (a separate scoped map), or the
                  pre-running states. Occupies the pane only when one of these is
                  the active view; otherwise null so the kept-alive panes below
                  fill it. Suppressed while the mobile guide tab is showing. */}
              {!(narrow && rightTab === "guide") &&
                (rightTab === "ai" && sessionId && (status === "running" || status === "starting") ? (
                // The panel mints/shows/revokes the scoped grant; anything the AI
                // runs shows up in the live Terminal tab, not here. Rendered while
                // still "starting" too (not just running) so the student can hand
                // their agent a key up front - the backend mints it during boot,
                // and the agent polls /ai/context's `ready` flag until exec works.
                // The panel shows a "still booting" note in that window.
                <AiAssistPanel
                  sessionId={sessionId}
                  provisioning={status !== "running"}
                  research={module?.track === "research"}
                />
              ) : rightTab === "topology" ? (
                sessionId ? (
                  // Scoped to this session and ownership-gated server-side -
                  // the student sees their own disposable cluster and one
                  // opaque node for everything else.
                  <ScopedTopologyMap
                    sessionId={sessionId}
                    moduleName={module?.displayName ?? slug ?? "lab"}
                  />
                ) : (
                  startPlaceholder
                )
              ) : !sessionId ? (
                startPlaceholder
              ) : status === "error" || status === "ended" || status === "gone" ? (
                <div className="flex h-full flex-col items-center justify-center gap-3 px-6 text-center text-sm">
                  {/* Tone carries the split: red only for a real failure, calm slate
                      for a normal end (see provisionFailed's note). */}
                  <span className={provisionFailed ? "text-red-400" : "text-slate-400"}>
                    {provisionFailed
                      ? "Couldn’t start this lab - our side, not yours."
                      : endedMessage(session)}
                  </span>
                  {/* A failed start is on our side, not something the student can
                      read a k8s error and fix - so we name the gate plainly and hand
                      them a way to reach us instead of dumping the raw provisioning
                      error. A reaped/ended lab is normal and needs no such line. */}
                  {provisionFailed && (
                    <span className="max-w-lg text-slate-500">
                      Pod scheduling failed
                      <TelegramLink />
                    </span>
                  )}
                  {/* One click, not two: the dead session is cleaned up as part of
                      starting a fresh one, so there's no separate "stop" step. A
                      normal end just needs a quiet "Start again" (ghost); a failure
                      gets the primary emphasis on retrying. */}
                  <Button
                    variant={provisionFailed ? "primary" : "ghost"}
                    onClick={() => retry.mutate()}
                    disabled={retry.isPending}
                    className="px-5"
                  >
                    {retry.isPending
                      ? provisionFailed
                        ? "Restarting…"
                        : "Starting…"
                      : provisionFailed
                        ? "Try again"
                        : "Start again"}
                  </Button>
                  {retry.isError && (
                    <span className="max-w-sm text-xs text-red-400">
                      Failed to restart lab
                      <TelegramLink />
                    </span>
                  )}
                </div>
              ) : status !== "running" ? (
                // Fade the loader in rather than hard-cutting to it, so starting a
                // lab (and the jump to this tab) reads as a calm transition.
                <div className="cd-fade h-full">
                  <ProvisioningLoader seed={sessionId ?? undefined} phase={session?.phase} module={slug} />
                </div>
              ) : null)}

              {/* Terminal + Defender each hold a live websocket into the session,
                  so once opened they stay MOUNTED and are only hidden on tab
                  switch (display:none) - flipping tabs is instant instead of
                  reconnecting (a fresh exec shell, or the Defender's ~20s
                  waitForDefenderPod). Mounted lazily on first visit (openedTabs)
                  so a session the student never inspects opens no extra streams. */}
              {sessionId && status === "running" && (
                <>
                  {openedTabs.has("terminal") && (
                    // The shell lives in a pod inside the session's nested cluster,
                    // so the terminal is an xterm over an exec websocket (see
                    // NestedTerminal). There is no ttyd to iframe any more - that
                    // path went with the legacy pod shape (F1).
                    <div className={rightTab === "terminal" ? "h-full" : "hidden"}>
                      <NestedTerminal sessionId={sessionId} />
                    </div>
                  )}
                  {module?.defender && openedTabs.has("defender") && (
                    // The in-guest sensor's live feed - the student watches their
                    // own lab being observed at the kernel level (see DefenderPane
                    // / internal/labs/defender.go). Modules with a Defender pane.
                    <div className={rightTab === "defender" ? "h-full" : "hidden"}>
                      <DefenderPane sessionId={sessionId} />
                    </div>
                  )}
                </>
              )}
            </div>
          </>
        }
      >
        {/* Desktop: the course pane lives in the split. On mobile it moves into
            the right pane as the "guide" tab (below), and the split is collapsed,
            so here it's null. */}
        {narrow ? null : courseSection}
      </ResizablePane>
    </div>
    </TerminalRunContext.Provider>
  );
}
