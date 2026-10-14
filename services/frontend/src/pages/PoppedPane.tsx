import { useEffect, useState } from "react";
import { useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import NestedTerminal from "../components/NestedTerminal";
import DefenderPane from "../components/DefenderPane";
import { getModules, getSession } from "../lib/api";

// A right-pane popped out into its own window (opened from the pane's pop-out
// button). On a small laptop, resizing the course pane against the terminal every
// time is the pain this removes: put the live pane in a second window and the
// theory in the first. `pane` selects which one - the terminal, or the read-only
// eBPF sensor feed - so the pop-out mirrors whichever tab the user was viewing
// (the same way the enlarge toggle expands whichever pane is active).
//
// It reuses LabPage's per-lab stored-session key, so this window binds to the
// SAME running session as the main one - localStorage is shared across
// same-origin windows, and each pane here is just a second stream into the same
// session (k8s allows concurrent execs; the sensor feed is read-only), so both
// windows stay live.
type Pane = "terminal" | "defender";

const sessionKey = (slug: string) => `k8labs:session:${slug}`;

function storedSessionId(slug: string): string | null {
  try {
    const raw = localStorage.getItem(sessionKey(slug));
    if (!raw) return null;
    return (JSON.parse(raw) as { id?: string }).id ?? null;
  } catch {
    return null;
  }
}

const LABEL: Record<Pane, string> = { terminal: "terminal", defender: "eBPF sensor" };

export default function PoppedPane({ pane }: { pane: Pane }) {
  const { slug } = useParams<{ slug: string }>();
  const [sessionId, setSessionId] = useState<string | null>(() =>
    slug ? storedSessionId(slug) : null,
  );

  // The main window can start (or restart) the lab after this popup is already
  // open, so listen for the stored id changing rather than freezing on whatever
  // was there at mount. `storage` fires in other-window tabs of the same origin.
  useEffect(() => {
    if (!slug) return;
    const onStorage = () => setSessionId(storedSessionId(slug));
    window.addEventListener("storage", onStorage);
    return () => window.removeEventListener("storage", onStorage);
  }, [slug]);

  const { data: modules } = useQuery({ queryKey: ["modules"], queryFn: getModules });
  const name = modules?.find((m) => m.slug === slug)?.displayName ?? slug ?? "lab";

  // Same lifecycle read as LabPage: poll fast while it provisions, slowly once
  // running (so a reap or restart is picked up), and stop on a terminal state.
  const { data: session } = useQuery({
    queryKey: ["session", sessionId],
    queryFn: () => getSession(sessionId!),
    enabled: !!sessionId,
    refetchInterval: (query) => {
      const s = query.state.data;
      if (s === null) return false;
      if (s && (s.status === "error" || s.status === "ended")) return false;
      return s && s.status === "running" ? 15000 : 1000;
    },
  });
  const status = sessionId
    ? session === null
      ? "gone"
      : (session?.status ?? "starting")
    : "none";

  useEffect(() => {
    document.title = `${name} ${LABEL[pane]}`;
  }, [name, pane]);

  const running = status === "running";

  return (
    <div className="flex h-screen w-screen flex-col bg-surface text-slate-200">
      <div className="flex h-10 shrink-0 items-center gap-2 border-b border-slate-800 bg-slate-900 px-3 text-13">
        <span
          className={`h-1.5 w-1.5 rounded-full ${
            running
              ? "bg-emerald-500"
              : status === "starting"
                ? "dot-progress"
                : "bg-slate-600"
          }`}
        />
        <span className="font-medium text-slate-300">{name}</span>
        <span className="text-slate-600">{LABEL[pane]}</span>
        <span className="ml-auto hidden text-slate-600 sm:inline">
          Same session as the main window.
        </span>
      </div>
      <div className="min-h-0 flex-1">
        {running && sessionId ? (
          pane === "defender" ? (
            <DefenderPane sessionId={sessionId} />
          ) : (
            <NestedTerminal sessionId={sessionId} />
          )
        ) : (
          <div className="flex h-full items-center justify-center px-6 text-center text-sm text-slate-500">
            {status === "none" || status === "gone"
              ? `This lab isn't running. Start it from the main window, then reopen the ${LABEL[pane]}.`
              : status === "starting"
                ? `The lab is still booting - the ${LABEL[pane]} connects the moment it's ready.`
                : "This lab session has ended. Start it again from the main window."}
          </div>
        )}
      </div>
    </div>
  );
}
