import { useState, useEffect, useRef } from "react";
import { Accessibility, AArrowUp, AArrowDown, Sun, RotateCcw, X } from "lucide-react";

// Ported from our shared component set's footer. Self-contained: persists its prefs in
// localStorage and applies them to the document root (font scale + a high-
// contrast filter, see index.css .a11y-high-contrast). Theme classes remapped
// from the original's dark-*/primary-* tokens to k8 labs' slate/accent/hairline.

const STORAGE_KEY = "a11y_prefs";

interface A11yPrefs {
  fontSize: number;
  highContrast: boolean;
}

const DEFAULT_PREFS: A11yPrefs = { fontSize: 0, highContrast: false };

function loadPrefs(): A11yPrefs {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    return raw ? { ...DEFAULT_PREFS, ...JSON.parse(raw) } : DEFAULT_PREFS;
  } catch {
    return DEFAULT_PREFS;
  }
}

function applyPrefs(prefs: A11yPrefs) {
  const root = document.documentElement;
  root.style.fontSize = prefs.fontSize !== 0 ? `${100 + prefs.fontSize * 10}%` : "";
  root.classList.toggle("a11y-high-contrast", prefs.highContrast);
}

export default function AccessibilityWidget() {
  const [open, setOpen] = useState(false);
  const [prefs, setPrefs] = useState<A11yPrefs>(DEFAULT_PREFS);
  const panelRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    const saved = loadPrefs();
    setPrefs(saved);
    applyPrefs(saved);
  }, []);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    const onClickOutside = (e: MouseEvent) => {
      if (
        panelRef.current &&
        !panelRef.current.contains(e.target as Node) &&
        buttonRef.current &&
        !buttonRef.current.contains(e.target as Node)
      ) {
        setOpen(false);
      }
    };
    document.addEventListener("keydown", onKey);
    document.addEventListener("mousedown", onClickOutside);
    return () => {
      document.removeEventListener("keydown", onKey);
      document.removeEventListener("mousedown", onClickOutside);
    };
  }, [open]);

  const update = (next: Partial<A11yPrefs>) => {
    const merged = { ...prefs, ...next };
    setPrefs(merged);
    applyPrefs(merged);
    localStorage.setItem(STORAGE_KEY, JSON.stringify(merged));
  };

  const reset = () => update(DEFAULT_PREFS);

  const isDefault = prefs.fontSize === 0 && !prefs.highContrast;

  return (
    <div className="relative">
      <button
        ref={buttonRef}
        onClick={() => setOpen((v) => !v)}
        aria-label="Accessibility options"
        aria-expanded={open}
        aria-haspopup="dialog"
        className="relative rounded p-0.5 text-slate-500 transition-colors hover:text-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
      >
        <Accessibility className="h-4 w-4" aria-hidden="true" />
        {!isDefault && (
          <span
            className="absolute -right-0.5 -top-0.5 h-1.5 w-1.5 rounded-full bg-accent"
            aria-hidden="true"
          />
        )}
      </button>

      {open && (
        <div
          ref={panelRef}
          role="dialog"
          aria-label="Accessibility options"
          aria-modal="false"
          className="animate-slide-up absolute bottom-8 right-0 z-50 w-52 rounded-lg border border-hairline bg-surface p-3 shadow-2xl"
        >
          <div className="mb-3 flex items-center justify-between">
            <span className="text-xs font-medium text-slate-200">Accessibility</span>
            <button
              onClick={() => setOpen(false)}
              aria-label="Close accessibility options"
              className="rounded text-slate-500 transition-colors hover:text-white focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
            >
              <X className="h-3 w-3" aria-hidden="true" />
            </button>
          </div>

          <div className="space-y-3">
            <div>
              <p className="mb-1.5 text-xs text-slate-500">Text size</p>
              <div className="flex items-center gap-2">
                <button
                  onClick={() => update({ fontSize: Math.max(-2, prefs.fontSize - 1) })}
                  disabled={prefs.fontSize <= -2}
                  aria-label="Decrease text size"
                  className="flex flex-1 items-center justify-center gap-1 rounded border border-hairline py-1 text-xs text-slate-500 transition-colors hover:border-accent hover:text-white focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent disabled:cursor-not-allowed disabled:opacity-30"
                >
                  <AArrowDown className="h-3.5 w-3.5" aria-hidden="true" />
                </button>
                <span
                  className="w-8 text-center text-xs text-slate-500"
                  aria-live="polite"
                  aria-label={`Text size ${prefs.fontSize > 0 ? `+${prefs.fontSize}` : prefs.fontSize}`}
                >
                  {prefs.fontSize > 0 ? `+${prefs.fontSize}` : prefs.fontSize}
                </span>
                <button
                  onClick={() => update({ fontSize: Math.min(3, prefs.fontSize + 1) })}
                  disabled={prefs.fontSize >= 3}
                  aria-label="Increase text size"
                  className="flex flex-1 items-center justify-center gap-1 rounded border border-hairline py-1 text-xs text-slate-500 transition-colors hover:border-accent hover:text-white focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent disabled:cursor-not-allowed disabled:opacity-30"
                >
                  <AArrowUp className="h-3.5 w-3.5" aria-hidden="true" />
                </button>
              </div>
            </div>

            <div className="flex items-center justify-between">
              <span className="text-xs text-slate-500">High contrast</span>
              <button
                onClick={() => update({ highContrast: !prefs.highContrast })}
                role="switch"
                aria-checked={prefs.highContrast}
                aria-label="Toggle high contrast mode"
                className={`relative h-4 w-8 rounded-full transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent ${
                  prefs.highContrast ? "bg-accent" : "bg-hairline"
                }`}
              >
                <span
                  className={`absolute left-0.5 top-0.5 h-3 w-3 rounded-full bg-white shadow transition-transform ${
                    prefs.highContrast ? "translate-x-4" : ""
                  }`}
                  aria-hidden="true"
                />
                <Sun className="sr-only" />
              </button>
            </div>

            {!isDefault && (
              <button
                onClick={reset}
                aria-label="Reset all accessibility settings to default"
                className="flex w-full items-center justify-center gap-1.5 rounded border border-hairline py-1 text-xs text-slate-500 transition-colors hover:border-accent hover:text-white focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
              >
                <RotateCcw className="h-3 w-3" aria-hidden="true" />
                Reset
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
