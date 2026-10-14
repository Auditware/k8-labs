import { useEffect, useState } from "react";

// The session countdown, driven entirely by server-supplied deadlines (see
// api.ts's Session.expiresAt / idleExpiresAt). Nothing here is derived from a
// locally remembered start time: that would drift, wouldn't survive a device
// change, and could be edited by the student - unacceptable for a number that
// claims to say when their work disappears.
//
// Two clocks, because the session dies to whichever fires first:
//   - expiresAt is the hard cap, fixed when the session was created. This is
//     the promise ("this lab is 60 minutes") and so it is the main readout.
//   - idleExpiresAt slides forward on every keystroke in the terminal. A
//     student reading the course for twenty minutes without typing is idle by
//     this measure, so showing only the hard cap would let a session vanish
//     with half an hour apparently still on the clock. Hence the warning.

const IDLE_WARN_MS = 3 * 60 * 1000;

function useNow(intervalMs = 1000) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), intervalMs);
    return () => clearInterval(id);
  }, [intervalMs]);
  return now;
}

// mm:ss up to an hour, then h:mm:ss. Clamped at zero - a session past its
// deadline is being reaped right now, and a negative countdown reads as a bug.
function formatRemaining(ms: number): string {
  const total = Math.max(0, Math.floor(ms / 1000));
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  const pad = (n: number) => String(n).padStart(2, "0");
  return h > 0 ? `${h}:${pad(m)}:${pad(s)}` : `${pad(m)}:${pad(s)}`;
}

export default function SessionTimer({
  expiresAt,
  idleExpiresAt,
  onExtend,
  extending,
}: {
  expiresAt?: string;
  idleExpiresAt?: string;
  // When provided (research labs only), render an "extend the hard cap" button.
  // The parent owns the request and the countdown refresh; this component just
  // surfaces the affordance next to the number it applies to.
  onExtend?: () => void;
  extending?: boolean;
}) {
  const now = useNow();
  if (!expiresAt) return null;

  const hardMs = new Date(expiresAt).getTime() - now;
  // A malformed or missing timestamp must not render "NaN:NaN".
  if (Number.isNaN(hardMs)) return null;

  const idleMs = idleExpiresAt ? new Date(idleExpiresAt).getTime() - now : Number.POSITIVE_INFINITY;
  // Only warn when idle is both imminent AND the thing that will actually kill
  // the session first - near the very end of a lab the hard cap wins anyway and
  // an idle warning would just be noise.
  const idleWarning = !Number.isNaN(idleMs) && idleMs < IDLE_WARN_MS && idleMs < hardMs;

  const expired = hardMs <= 0;
  const lowTime = hardMs < 5 * 60 * 1000;

  return (
    <span className="flex items-center gap-2">
      <span
        className={`font-mono text-xs tabular-nums ${
          expired || lowTime ? "text-amber-400" : "text-slate-500"
        }`}
        title={`Session ends at ${new Date(expiresAt).toLocaleTimeString()}`}
      >
        {expired ? "expired" : `${formatRemaining(hardMs)} left`}
      </span>
      {idleWarning && !expired && (
        <span
          className="rounded bg-amber-500/10 px-1.5 py-0.5 font-mono text-10 text-amber-400 ring-1 ring-inset ring-amber-500/30"
          title="This session is reaped after a period with no terminal input, separately from the countdown. Type in the terminal to keep it alive."
        >
          idle {formatRemaining(idleMs)}
        </span>
      )}
      {onExtend && !expired && (
        <button
          type="button"
          onClick={onExtend}
          disabled={extending}
          className={`rounded px-1.5 py-0.5 font-mono text-10 ring-1 ring-inset transition-colors disabled:opacity-50 ${
            lowTime
              ? "bg-amber-500/10 text-amber-400 ring-amber-500/30 hover:bg-amber-500/20"
              : "text-slate-400 ring-slate-700 hover:bg-slate-800 hover:text-slate-200"
          }`}
          title="Add 2 hours to this research session (up to a 48-hour maximum)."
        >
          {extending ? "extending…" : "+2h"}
        </button>
      )}
    </span>
  );
}
