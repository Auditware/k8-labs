import { useEffect, useMemo, useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { createAiGrant, revokeAiGrant, type AiGrant } from "../../lib/api";
import { Button } from "../ui/Form";

// The AI lab-assist tab (design: "security brief"). It mints a short-lived,
// session-scoped grant the student pastes into their own AI, so the AI can read
// the lesson/topology/terminal and run commands - which appear on the student's
// own Terminal tab. It cannot submit flags or stop the lab (enforced server-side;
// see internal/api/ai.go). Access ends at the earliest of the 15-minute expiry,
// the lab closing, or the student clicking Revoke - deliberately NOT on merely
// switching tabs, since the student watches the AI work from the Terminal tab.
export default function AiAssistPanel({
  sessionId,
  provisioning = false,
  research = false,
}: {
  sessionId: string;
  provisioning?: boolean;
  // research = a kernel-driver research lab: a discovery bench with no course and
  // no flag, so the copy drops the tutor "it coaches, won't solve it" framing and
  // describes a plain research assistant instead (the pasted prompt differs too,
  // server-side in internal/api/ai.go).
  research?: boolean;
}) {
  const grantKey = `k8labs:aigrant:${sessionId}`;
  const [grant, setGrant] = useState<AiGrant | null>(null);
  const [copied, setCopied] = useState<string | null>(null);
  const [remaining, setRemaining] = useState(0);

  const create = useMutation({
    mutationFn: () => createAiGrant(sessionId),
    onSuccess: (g) => {
      setGrant(g);
      localStorage.setItem(grantKey, JSON.stringify(g));
    },
  });
  const revoke = useMutation({
    mutationFn: () => revokeAiGrant(sessionId),
    onSuccess: () => {
      setGrant(null);
      localStorage.removeItem(grantKey);
    },
  });

  // Live countdown to the grant's expiry.
  useEffect(() => {
    if (!grant) return;
    const tick = () =>
      setRemaining(Math.max(0, Math.round((Date.parse(grant.expiresAt) - Date.now()) / 1000)));
    tick();
    const h = window.setInterval(tick, 1000);
    return () => window.clearInterval(h);
  }, [grant]);

  // Reload an active grant on mount so switching tabs (this panel unmounts when
  // it is not the active tab) or reloading the page keeps the SAME key showing -
  // and, crucially, does NOT revoke it. The AI has to keep working while the
  // student is over on the Terminal tab watching it. That was the bug: revoking
  // on unmount killed the AI's token the moment the student left this tab.
  // Revocation is now explicit only: the Revoke button, lab teardown
  // (server-side), or the 15-minute expiry.
  useEffect(() => {
    const raw = localStorage.getItem(grantKey);
    if (!raw) return;
    try {
      const g = JSON.parse(raw) as AiGrant;
      if (Date.parse(g.expiresAt) > Date.now()) setGrant(g);
      else localStorage.removeItem(grantKey);
    } catch {
      localStorage.removeItem(grantKey);
    }
  }, [grantKey]);

  const expired = grant != null && remaining <= 0;

  const configBlob = useMemo(() => {
    if (!grant) return "";
    return JSON.stringify(
      {
        baseUrl: grant.baseUrl,
        token: grant.token,
        expiresAt: grant.expiresAt,
        instructions: grant.miniPrompt,
        api: grant.api,
      },
      null,
      2,
    );
  }, [grant]);

  const copy = (label: string, text: string) => {
    navigator.clipboard?.writeText(text).then(
      () => {
        setCopied(label);
        window.setTimeout(() => setCopied((c) => (c === label ? null : c)), 1500);
      },
      () => {},
    );
  };

  const mmss = (s: number) => `${Math.floor(s / 60)}:${String(s % 60).padStart(2, "0")}`;

  return (
    <div className="flex h-full items-center justify-center overflow-y-auto bg-surface px-5 py-8 text-slate-200">
      {!grant ? (
        // --- Empty state: the "security brief" ---
        <div className="w-full max-w-md text-center">
          <ProvisioningNote show={provisioning} />
          <div className="mx-auto grid h-12 w-12 place-items-center rounded-xl border border-slate-700 bg-gradient-to-b from-slate-800/70 to-slate-900 text-indigo-300">
            <AgentIcon />
          </div>
          <h2 className="mt-4 text-lg font-semibold text-slate-100">Bring your own AI</h2>
          <p className="mx-auto mt-1.5 max-w-sm text-13 leading-relaxed text-slate-400">
            {research ? (
              <>
                A scoped key into <em className="text-slate-200 not-italic">this</em> session -
                mirrored terminal access so your own AI can research alongside you.
              </>
            ) : (
              <>
                A scoped, 15-minute key into <em className="text-slate-200 not-italic">this</em> lab.
                It coaches - it won&rsquo;t solve it for you.
              </>
            )}
          </p>

          <div className="mt-6">
            <Button onClick={() => create.mutate()} disabled={create.isPending}>
              {create.isPending ? "Generating…" : "Generate AI access key"}
            </Button>
            {create.isError && (
              <p className="mt-2 text-xs text-red-400">
                Couldn&rsquo;t generate a key. Make sure the lab is still running, then try again.
              </p>
            )}
          </div>
        </div>
      ) : (
        // --- Active state: key + config, same visual language ---
        <div className="w-full max-w-md text-center">
          <ProvisioningNote show={provisioning && !expired} />
          <div
            className={`mx-auto inline-flex items-center gap-2 rounded-full border px-3 py-1 text-xs ${
              expired
                ? "border-red-500/30 bg-red-500/10 text-red-300"
                : "border-emerald-500/30 bg-emerald-500/10 text-emerald-300"
            }`}
          >
            {expired ? (
              "Key expired"
            ) : (
              <>
                <span className="font-mono tabular-nums">{mmss(remaining)}</span>
                <span className="opacity-80">active &middot; mirrored to your Terminal</span>
              </>
            )}
          </div>

          {!expired && (
            <div className="mt-4 text-left">
              <span className="mb-1 block text-xs uppercase tracking-wide text-slate-500">
                Config for your AI
              </span>
              <pre className="max-h-56 overflow-auto rounded-lg border border-slate-800 bg-slate-950 p-3 text-left text-xs leading-relaxed text-slate-300">
                <HighlightedJson text={configBlob} />
              </pre>
            </div>
          )}

          {/* Exactly three actions: Copy (the config, the one thing you do with
              it), Regenerate, Revoke. */}
          <div className="mt-5 flex justify-center gap-2">
            {!expired && (
              <Button size="sm" onClick={() => copy("config", configBlob)}>
                {copied === "config" ? "Copied" : "Copy"}
              </Button>
            )}
            <Button size="sm" variant="ghost" onClick={() => create.mutate()} disabled={create.isPending}>
              {create.isPending ? "Regenerating…" : expired ? "Generate a new key" : "Regenerate"}
            </Button>
            <Button size="sm" variant="ghost" onClick={() => revoke.mutate()} disabled={revoke.isPending}>
              {revoke.isPending ? "Revoking…" : "Revoke"}
            </Button>
          </div>
          <p className="mx-auto mt-3 max-w-sm text-xs text-slate-500">
            Regenerating invalidates the previous key instantly. The key also dies when the lab
            stops or you leave this tab.
          </p>
        </div>
      )}
    </div>
  );
}

// A capability chip: a small reassurance of what the AI can (ok) or cannot (x) do.
// A slim banner shown while the lab is still booting: the AI can be handed a key
// and start reading the lesson now; commands run once the lab is ready (the API's
// `ready` flag). Keeps the tab useful during the ~20s provision instead of a bare
// spinner, and matches the "generate up front, poll for liveness" flow.
function ProvisioningNote({ show }: { show: boolean }) {
  if (!show) return null;
  return (
    <div className="mx-auto mb-4 flex max-w-sm items-center justify-center gap-2 rounded-md border border-amber-500/30 bg-amber-500/10 px-3 py-1.5 text-xs text-amber-300">
      <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-amber-400" />
      Lab is still booting - your AI can read the lesson now; commands run once it&rsquo;s
      ready.
    </div>
  );
}

// Minimal, dependency-free JSON syntax highlighter. The config is server-built
// JSON (no user input), and we render React spans rather than injecting HTML, so
// there's no escaping/XSS surface. Keys sky, string values emerald, numbers
// amber, literals violet; punctuation stays in the base color.
function HighlightedJson({ text }: { text: string }) {
  const re = /("(?:\\.|[^"\\])*")(\s*:)?|(-?\d+\.?\d*)|\b(true|false|null)\b/g;
  const out: React.ReactNode[] = [];
  let last = 0;
  let i = 0;
  let m: RegExpExecArray | null;
  while ((m = re.exec(text)) !== null) {
    if (m.index > last) out.push(text.slice(last, m.index));
    if (m[1] && m[2] !== undefined) {
      out.push(
        <span key={i++} className="text-sky-300">
          {m[1]}
        </span>,
      );
      out.push(
        <span key={i++} className="text-slate-500">
          {m[2]}
        </span>,
      );
    } else if (m[1]) {
      out.push(
        <span key={i++} className="text-emerald-300">
          {m[1]}
        </span>,
      );
    } else if (m[3]) {
      out.push(
        <span key={i++} className="text-amber-300">
          {m[3]}
        </span>,
      );
    } else if (m[4]) {
      out.push(
        <span key={i++} className="text-violet-300">
          {m[4]}
        </span>,
      );
    }
    last = re.lastIndex;
  }
  if (last < text.length) out.push(text.slice(last));
  return <>{out}</>;
}

// A little agent/robot, in place of the old shield - this is an AI assistant,
// not a security badge.
function AgentIcon() {
  return (
    <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" strokeWidth="1.6">
      <rect x="4" y="8" width="16" height="12" rx="3" />
      <path d="M12 4.5V8" />
      <circle cx="12" cy="3.3" r="1.2" fill="currentColor" stroke="none" />
      <circle cx="9" cy="13" r="1.25" fill="currentColor" stroke="none" />
      <circle cx="15" cy="13" r="1.25" fill="currentColor" stroke="none" />
      <path d="M9.5 16.5h5" strokeLinecap="round" />
      <path d="M4 12H2.5M20 12h1.5" strokeLinecap="round" />
    </svg>
  );
}
