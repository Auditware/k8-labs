import { useEffect, useRef } from "react";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import { terminalWsUrl } from "../lib/api";
import { useTerminalRun } from "./course/terminalRun";

// The terminal for nested-shape modules. A nested session has no ttyd to
// iframe; the shell lives in a pod inside the disposable nested cluster and the
// backend execs into it over one websocket (see internal/labs/terminal.go).
// This renders xterm.js and speaks that protocol:
//   - keystrokes -> binary frames (stdin)
//   - PTY size   -> text frame {"cols":N,"rows":M} (resize)
//   - server     -> binary frames (stdout), written straight to the terminal.
export default function NestedTerminal({ sessionId }: { sessionId: string }) {
  const ref = useRef<HTMLDivElement>(null);
  const run = useTerminalRun();

  useEffect(() => {
    const el = ref.current;
    if (!el) return;

    const term = new Terminal({
      convertEol: true,
      cursorBlink: true,
      fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace",
      fontSize: 17,
      // The prompt (see internal/labs/terminal.go's PS1) colors itself with
      // ANSI codes, so pin the palette here rather than trusting xterm's
      // defaults - the default blue (#2472c8) is nearly the background hue and
      // reads as mud. These are chosen against the #0b1120 navy: a bright cyan
      // name, a dim-gray path, and green/red for the $/# sigil that flips on
      // escalation. brightBlack is the path gray; brightGreen/brightRed the
      // sigil. Only the slots the prompt actually uses are overridden.
      theme: {
        background: "#0b1120",
        foreground: "#c9d1d9",
        cyan: "#56b6c2",
        brightCyan: "#56b6c2",
        green: "#23d18b",
        brightGreen: "#23d18b",
        red: "#f14c4c",
        brightRed: "#f14c4c",
        brightBlack: "#8b949e",
      },
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(el);
    fit.fit();

    const enc = new TextEncoder();

    // The websocket is an exec shell into the session's foothold pod. It can drop
    // while the lab itself is still running - a backend redeploy, a proxy/idle
    // cut, or the shell exiting - so instead of stranding a dead "[session
    // closed]" over a live lab, reconnect (which opens a fresh shell) with a
    // short backoff, and only declare it closed after several failed tries. The
    // idle reaper + hard cap (see internal/labs) still stop a truly abandoned lab.
    let ws: WebSocket | null = null;
    let unregister: (() => void) | null = null;
    let disposed = false;
    let attempts = 0;
    // A reconnect status line is currently showing; the next real byte from the
    // shell clears it. Kept so the status updates IN PLACE (\r + clear-line)
    // rather than spamming a new "[reconnecting…]"/"[reconnected]" pair per flap.
    let statusShown = false;
    let retryTimer = 0;
    const MAX_RETRIES = 6;
    const CLEAR_LINE = "\r\x1b[2K"; // carriage return + erase the line

    const sendSize = (sock: WebSocket) => {
      if (sock.readyState === WebSocket.OPEN) {
        sock.send(JSON.stringify({ cols: term.cols, rows: term.rows }));
      }
    };

    const connect = () => {
      const sock = new WebSocket(terminalWsUrl(sessionId));
      sock.binaryType = "arraybuffer";
      ws = sock;
      sock.onopen = () => {
        // Don't announce "[reconnected]" here - a flapping session opens and
        // closes without ever delivering a byte, and announcing each open is
        // half the spam. The status line is cleared by the first real byte
        // (onmessage) instead, and attempts is reset there too.
        sendSize(sock);
        term.focus();
        // Expose this shell to the course's Run buttons while the socket is open,
        // so a command chip can paste itself in as if typed.
        unregister?.();
        unregister =
          run?.registerTerminal((data) => {
            if (sock.readyState === WebSocket.OPEN) sock.send(enc.encode(data));
          }) ?? null;
      };
      sock.onmessage = (ev) => {
        // First real byte after a reconnect: wipe the status line in place and
        // reset the counter, so the shell resumes cleanly on that same line.
        if (statusShown) {
          term.write(CLEAR_LINE);
          statusShown = false;
          attempts = 0;
        }
        if (typeof ev.data === "string") term.write(ev.data);
        else term.write(new Uint8Array(ev.data as ArrayBuffer));
      };
      sock.onclose = () => {
        if (disposed) return;
        unregister?.();
        unregister = null;
        attempts++;
        // One status line, updated in place: start it on its own line the first
        // time, then rewrite it (\r + clear) so repeated flaps just bump the
        // count - "[reconnecting… (2)]", "(3)", ... - instead of new lines.
        const prefix = statusShown ? CLEAR_LINE : "\r\n";
        if (attempts > MAX_RETRIES) {
          term.write(`${prefix}\x1b[2m[session closed]\x1b[0m\r\n`);
          statusShown = false;
          return;
        }
        term.write(`${prefix}\x1b[2m[reconnecting… (${attempts})]\x1b[0m`);
        statusShown = true;
        retryTimer = window.setTimeout(connect, Math.min(1000 * attempts, 5000));
      };
    };

    // Keystrokes as binary so they're never mistaken for a resize text frame.
    const onData = term.onData((data) => {
      if (ws && ws.readyState === WebSocket.OPEN) ws.send(enc.encode(data));
    });
    const onResize = term.onResize(() => {
      if (ws) sendSize(ws);
    });

    const ro = new ResizeObserver(() => {
      try {
        fit.fit();
      } catch {
        /* terminal not visible yet; next observe will refit */
      }
    });
    ro.observe(el);

    // Defer the first fit+connect one frame. In a pane that's already laid out
    // (the main lab split) this is imperceptible; in a freshly-opened window
    // (the popped-out terminal) the container reports ~0 width on the very first
    // paint, so fitting then connecting synchronously would size the PTY at ~1
    // column and the shell's opening prompt would wrap one char per line until a
    // later resize. One frame lets layout settle so the first prompt lands at
    // the real width.
    const start = requestAnimationFrame(() => {
      if (disposed) return;
      try {
        fit.fit();
      } catch {
        /* container not measurable yet; the ResizeObserver will refit */
      }
      connect();
    });

    return () => {
      disposed = true;
      cancelAnimationFrame(start);
      window.clearTimeout(retryTimer);
      unregister?.();
      ro.disconnect();
      onData.dispose();
      onResize.dispose();
      ws?.close();
      term.dispose();
    };
  }, [sessionId, run]);

  // A little breathing room so the first/last row isn't jammed against the pane
  // edge. FitAddon derives rows from the parent's content height, so the bottom
  // padding is what keeps the final prompt row off the very bottom edge (without
  // it the remainder gap can round to ~0 and the prompt reads as half-cut). The
  // wrapper's own navy background fills the inset seamlessly.
  return (
    <div ref={ref} className="h-full w-full bg-surface pb-3.5 pl-3.5 pt-3.5" />
  );
}
