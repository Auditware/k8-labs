import { useEffect, useRef, useState } from "react";
import { defenderWsUrl } from "../lib/api";

// The Defender pane for eBPF modules. A read-only view of the in-guest Tetragon
// sensor's live event feed, streamed over one websocket from the backend
// (internal/labs/defender.go). The eBPF is the product here: the student watches
// their OWN lab being observed at the kernel level, and sees the privilege
// escalation they just performed get caught below the shell - the euid jump that
// leaves no trace in the terminal.
//
// Tetragon emits one JSON object per line. We care about two shapes:
//   - process_exec: a program ran (binary + args).
//   - process_kprobe, function_name commit_creds: the kernel installed new
//     credentials. The policy captures the cred as process_credentials_arg.
//
// euid 0 alone is NOT the signal: the platform's own machinery (runc, k3s,
// containerd) installs root credentials constantly and legitimately - hundreds
// of events - but those processes are ALREADY root, so their real uid is 0 too
// (uid == euid == 0). A SUID privilege escalation is the one shape nothing else
// on the system produces: the effective uid jumps to 0 while the REAL uid stays
// the unprivileged user (the borrowed-euid trick backup-tool's `bash -p` keeps
// alive). That euid-0-but-uid-not-0 divergence is the student's own footprint.
// Verified against a live session: the escalation is uid=10001, euid=0; every
// system event is uid==euid (0/0 or 100/100). So we flag the divergence loudly
// as "escalation" and demote the uid==0 root churn to a muted row.

type Row = {
  key: number;
  time?: string;
  kind: "exec" | "escalation" | "creds-root" | "creds" | "raw";
  text: string;
};

// Tetragon's event JSON is loosely typed here - we read a few known fields
// defensively rather than model the whole schema.
type Ev = {
  time?: string;
  process_exec?: { process?: { binary?: string; arguments?: string } };
  process_kprobe?: {
    function_name?: string;
    // Tetragon attaches the tracing policy's `message` here (process_kprobe.message,
    // NOT top-level). ebpf-detect uses it to carry the flag, revealed only on the
    // escalation row below.
    message?: string;
    process?: { binary?: string };
    args?: Array<{ process_credentials_arg?: { uid?: number; euid?: number; suid?: number } }>;
  };
};

const MAX_ROWS = 250;

// Exported for tests: the parse is the load-bearing bit (turning Tetragon's
// loosely-typed JSON into a legible row, and spotting the euid-0 escalation).
export function parse(line: string, key: number): Row | null {
  let e: Ev;
  try {
    e = JSON.parse(line);
  } catch {
    return { key, kind: "raw", text: line.slice(0, 200) };
  }
  if (e.process_exec) {
    const p = e.process_exec.process;
    if (!p?.binary) return null;
    const args = p.arguments ? ` ${p.arguments}` : "";
    return { key, time: e.time, kind: "exec", text: `exec  ${p.binary}${args}` };
  }
  const pk = e.process_kprobe;
  if (pk && pk.function_name === "commit_creds") {
    const cred = pk.args?.map((a) => a.process_credentials_arg).find(Boolean);
    const euid = cred?.euid;
    const uid = cred?.uid;
    const bin = pk.process?.binary && pk.process.binary !== "" ? pk.process.binary : "a process";
    // Show the full uid triple the sensor reported - real, effective, saved - so
    // the line carries the whole credential fact and hides nothing. This is what
    // actually distinguishes an escalation from system root churn: a SUID privesc
    // is euid 0 with the REAL uid still unprivileged (uid != euid), while the
    // platform's own root machinery (runc/k3s) is already root (uid == euid == 0).
    // With uid on the line the difference is legible even with the colour stripped
    // - the colour just draws the eye to it.
    const n = (v?: number) => (v === undefined ? "?" : String(v));
    const ids = `uid ${n(uid)}, euid ${n(euid)}, suid ${n(cred?.suid)}`;
    if (euid === 0) {
      const text = `commit_creds  root credentials installed (${ids}) - via ${bin}`;
      // euid 0 with a non-root real uid is the escalation (the student's own
      // footprint). If uid is absent we can't judge divergence, so we fall back to
      // the plain root-cred row rather than false-alarm.
      if (uid !== undefined && uid !== 0) {
        // The flag rides the event's message (see ebpf-detect's tracing policy).
        // Reveal it here and ONLY here - on the student's own escalation - so it
        // stays hidden through the system's uid==euid==0 root churn below.
        const flag = pk.message ? `${text}  ·  flag: ${pk.message}` : text;
        return { key, time: e.time, kind: "escalation", text: flag };
      }
      return { key, time: e.time, kind: "creds-root", text };
    }
    return { key, time: e.time, kind: "creds", text: `commit_creds  ${ids} - via ${bin}` };
  }
  return null;
}

function hhmmss(t?: string): string {
  if (!t) return "";
  const d = new Date(t);
  return isNaN(d.getTime()) ? "" : d.toLocaleTimeString([], { hour12: false });
}

export default function DefenderPane({ sessionId }: { sessionId: string }) {
  const [rows, setRows] = useState<Row[]>([]);
  const [status, setStatus] = useState<"connecting" | "live" | "closed">("connecting");
  const [query, setQuery] = useState("");
  const scrollRef = useRef<HTMLDivElement>(null);
  const keyRef = useRef(0);
  const pinnedRef = useRef(true); // auto-scroll while the user is at the bottom

  // Client-side substring filter over the buffered feed - the whole point of a
  // firehose sensor is being able to grep it for the one syscall/binary/uid you
  // care about. Case-insensitive over the rendered text.
  const q = query.trim().toLowerCase();
  const shown = q ? rows.filter((r) => r.text.toLowerCase().includes(q)) : rows;

  useEffect(() => {
    setRows([]);
    setStatus("connecting");
    const ws = new WebSocket(defenderWsUrl(sessionId));

    ws.onopen = () => setStatus("live");
    ws.onmessage = (ev) => {
      if (typeof ev.data !== "string") return;
      const row = parse(ev.data, keyRef.current++);
      if (!row) return;
      setRows((prev) => {
        const next = prev.length >= MAX_ROWS ? prev.slice(prev.length - MAX_ROWS + 1) : prev;
        return [...next, row];
      });
    };
    ws.onclose = () => setStatus("closed");
    ws.onerror = () => setStatus("closed");

    return () => ws.close();
  }, [sessionId]);

  // Auto-scroll to the newest row, but only if the user hasn't scrolled up to
  // read history (otherwise the feed would yank them back down on every event).
  useEffect(() => {
    const el = scrollRef.current;
    if (el && pinnedRef.current) el.scrollTop = el.scrollHeight;
  }, [rows]);

  const onScroll = () => {
    const el = scrollRef.current;
    if (!el) return;
    pinnedRef.current = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
  };

  return (
    <div className="flex h-full flex-col bg-surface">
      <div className="flex shrink-0 items-center gap-2 border-b border-slate-800 px-4 py-2 text-11 text-slate-500">
        <span
          className={`h-1.5 w-1.5 rounded-full ${
            status === "live" ? "bg-emerald-500" : status === "closed" ? "bg-red-500" : "bg-amber-500"
          }`}
        />
        <span className="uppercase tracking-wide">
          eBPF sensor {status === "live" ? "· live" : status === "closed" ? "· disconnected" : "· connecting"}
        </span>
        <input
          type="text"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="filter events…"
          spellCheck={false}
          className="ml-auto w-40 rounded border border-slate-800 bg-slate-950 px-2 py-0.5 font-mono text-11 text-slate-300 placeholder:text-slate-600 focus:border-slate-600 focus:outline-none"
        />
        {q && (
          <span className="shrink-0 tabular-nums text-slate-600">
            {shown.length}/{rows.length}
          </span>
        )}
      </div>
      <div
        ref={scrollRef}
        onScroll={onScroll}
        className="min-h-0 flex-1 overflow-auto px-3 py-2 font-mono text-xs leading-relaxed"
      >
        {rows.length === 0 ? (
          <div className="flex h-full items-center justify-center px-6 text-center text-slate-600">
            {status === "closed"
              ? "Sensor feed disconnected."
              : "Waiting for kernel events… work in the terminal to see the kernel react."}
          </div>
        ) : shown.length === 0 ? (
          <div className="flex h-full items-center justify-center px-6 text-center text-slate-600">
            No events match “{query.trim()}”.
          </div>
        ) : (
          shown.map((r) => (
            <div
              key={r.key}
              className={`flex gap-2 whitespace-pre-wrap break-all ${
                r.kind === "escalation" ? "rounded bg-rose-950/50 px-1 ring-1 ring-rose-500/40" : ""
              }`}
            >
              <span className="shrink-0 text-slate-600">{hhmmss(r.time)}</span>
              <span
                className={
                  r.kind === "escalation"
                    ? "font-semibold text-rose-300"
                    : r.kind === "creds-root"
                      ? "text-slate-500"
                      : r.kind === "creds"
                        ? "text-slate-500"
                        : r.kind === "exec"
                          ? "text-slate-300"
                          : "text-slate-600"
                }
              >
                {r.kind === "escalation" ? `⚠ ${r.text}` : r.text}
              </span>
            </div>
          ))
        )}
      </div>
    </div>
  );
}
