import { useCallback, useEffect, useRef, useState } from "react";

// A resizable split between the course pane and the terminal.
//
// HORIZONTAL on wide screens: drag the width, course on the left. VERTICAL on
// narrow ones: the panes stack, course on top, and you drag the height. Without
// the flip a phone-width host squeezes the right pane down to an unusable,
// overflowing sliver - so below the width that fits both panes at their minimum
// we switch orientation (a ResizeObserver on the host, so it also reacts to the
// sidebar collapsing, not just window resize).
//
// The size is remembered in localStorage - a per-device display preference that
// means nothing on another machine, so losing it costs one drag. Width and
// height are stored separately, so each orientation keeps its own preference.
//
// Pointer events (not mouse) so trackpad, touch and stylus share one path; the
// drag runs on WINDOW listeners bound synchronously in pointerdown, which (a)
// survives the parent re-rendering every 2s while a lab provisions - a
// handle-scoped handler drops the drag on that tick - and (b) can't leave the
// handle stuck "dragging" when a bare click's pointerup beats an effect.

const H_STORE = "k8labs:coursePaneWidth";
const V_STORE = "k8labs:coursePaneHeight";
// Each pane's floor. Horizontal reserves the same MIN on both sides so the
// divider stops symmetrically; vertical uses a smaller floor because phones are
// short and 390px each would leave almost nothing for the second pane.
const H_MIN = 390;
const V_MIN = 160;
const MAX = 1600;
const H_DEFAULT = 480;
const V_DEFAULT = 320;
// Flip to vertical once the host can't give both panes their horizontal MIN.
const VERTICAL_BELOW = 2 * H_MIN;

function readStored(key: string, min: number, def: number): number {
  // No storage during server-side rendering - fall back to the default there.
  if (typeof window === "undefined") return def;
  try {
    const raw = Number(localStorage.getItem(key));
    // Clamp on read, not just on write: a size stored on a bigger screen could
    // otherwise leave the other pane invisible here.
    if (Number.isFinite(raw) && raw >= min && raw <= MAX) return raw;
  } catch {
    // Private mode / blocked storage - fall through to the default.
  }
  return def;
}

export default function ResizablePane({
  children,
  right,
  collapsed,
}: {
  children: React.ReactNode;
  right: React.ReactNode;
  collapsed?: boolean;
}) {
  const hostRef = useRef<HTMLDivElement>(null);
  // Seed from the viewport so a phone renders vertical on the first paint (no
  // horizontal flash); the effect below then corrects from the real host width.
  const [vertical, setVertical] = useState(
    () => typeof window !== "undefined" && window.innerWidth < VERTICAL_BELOW + 260,
  );
  const cfg = vertical
    ? { store: V_STORE, min: V_MIN, def: V_DEFAULT }
    : { store: H_STORE, min: H_MIN, def: H_DEFAULT };

  const [size, setSize] = useState(() => readStored(H_STORE, H_MIN, H_DEFAULT));
  // Latest size, read by the pointerup that persists it - a ref so the
  // window-scoped listener always sees the current value without re-binding.
  const sizeRef = useRef(size);

  // Orientation follows the host's own width (not the viewport), so collapsing
  // the sidebar - which widens the host - can flip us back to horizontal.
  // Measure directly on mount and on window resize (both reliable everywhere),
  // and observe the host too so a sidebar toggle that changes width without a
  // window resize still reorients.
  useEffect(() => {
    const host = hostRef.current;
    if (!host) return;
    const measure = () => setVertical(host.clientWidth < VERTICAL_BELOW);
    measure();
    let ro: ResizeObserver | undefined;
    if (typeof ResizeObserver !== "undefined") {
      ro = new ResizeObserver(measure);
      ro.observe(host);
    }
    window.addEventListener("resize", measure);
    return () => {
      ro?.disconnect();
      window.removeEventListener("resize", measure);
    };
  }, []);

  // When orientation flips, load that axis's own stored size.
  useEffect(() => {
    const v = readStored(cfg.store, cfg.min, cfg.def);
    setSize(v);
    sizeRef.current = v;
    // cfg is derived from `vertical`; re-run only when orientation changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [vertical]);

  const [dragging, setDragging] = useState(false);

  const clamp = useCallback(
    (px: number) => {
      const host = hostRef.current;
      // Reserve the same MIN for the far pane so the divider stops
      // symmetrically; MAX caps the near pane on ultrawide screens.
      const extent = host ? (vertical ? host.clientHeight : host.clientWidth) : Infinity;
      const ceiling = Math.min(MAX, extent - cfg.min);
      return Math.max(cfg.min, Math.min(ceiling, px));
    },
    [vertical, cfg.min],
  );

  // Keep sizeRef mirroring size so a drag that ends WITHOUT a move (a bare
  // click), or right after a keyboard/orientation change, persists correctly.
  useEffect(() => {
    sizeRef.current = size;
  }, [size]);

  const onPointerDown = (e: React.PointerEvent<HTMLDivElement>) => {
    // Don't let the press start a text selection; the drag is handled on window.
    e.preventDefault();
    setDragging(true);
    const onMove = (ev: PointerEvent) => {
      const host = hostRef.current;
      if (!host) return;
      const rect = host.getBoundingClientRect();
      const pos = vertical ? ev.clientY - rect.top : ev.clientX - rect.left;
      const next = clamp(pos);
      sizeRef.current = next;
      setSize(next);
    };
    const onUp = () => {
      setDragging(false);
      window.removeEventListener("pointermove", onMove);
      window.removeEventListener("pointerup", onUp);
      window.removeEventListener("pointercancel", onUp);
      try {
        localStorage.setItem(cfg.store, String(sizeRef.current));
      } catch {
        // Nothing to do - the size just won't survive a reload.
      }
    };
    window.addEventListener("pointermove", onMove);
    window.addEventListener("pointerup", onUp);
    window.addEventListener("pointercancel", onUp);
  };

  // Keyboard resizing, because a drag handle that only responds to a pointer is
  // unusable for anyone who doesn't use one. Arrows follow the drag axis.
  const onKeyDown = (e: React.KeyboardEvent<HTMLDivElement>) => {
    const step = (e.shiftKey ? 64 : 16) * (e.key === (vertical ? "ArrowUp" : "ArrowLeft") ? -1 : 1);
    const grows = e.key === (vertical ? "ArrowDown" : "ArrowRight");
    const shrinks = e.key === (vertical ? "ArrowUp" : "ArrowLeft");
    if (!grows && !shrinks) return;
    setSize((s) => clamp(s + step));
    e.preventDefault();
  };

  // A resize can invalidate the stored size (shrink the window and the far pane
  // vanishes), so re-clamp whenever the host changes.
  useEffect(() => {
    const onResize = () => setSize((s) => clamp(s));
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, [clamp]);

  return (
    <div ref={hostRef} className={`flex h-full min-h-0 w-full ${vertical ? "flex-col" : "flex-row"}`}>
      {!collapsed && (
        <>
          <section
            style={vertical ? { height: size } : { width: size }}
            className={`flex min-h-0 min-w-0 shrink-0 flex-col bg-slate-900 ${
              vertical ? "border-b" : "border-r"
            } border-slate-800`}
          >
            {children}
          </section>
          <div
            role="separator"
            aria-orientation={vertical ? "horizontal" : "vertical"}
            aria-label={vertical ? "Resize the course pane height" : "Resize the course pane"}
            aria-valuenow={Math.round(size)}
            aria-valuemin={cfg.min}
            aria-valuemax={MAX}
            tabIndex={0}
            onPointerDown={onPointerDown}
            onKeyDown={onKeyDown}
            style={{ touchAction: "none" }}
            className={`shrink-0 transition-colors focus:outline-none ${
              vertical ? "h-1.5 w-full cursor-row-resize" : "w-1.5 h-full cursor-col-resize"
            } ${
              dragging ? "bg-blue-500" : "bg-slate-800 hover:bg-slate-600 focus-visible:bg-blue-500"
            }`}
          />
        </>
      )}
      <section className="flex min-h-0 min-w-0 flex-1 flex-col bg-slate-950">{right}</section>
    </div>
  );
}
