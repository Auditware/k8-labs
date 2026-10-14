// The diagram renderer: a declarative shape list -> SVG.
//
// A port of the training-guide template's box/wire/lbl/seal helpers, with two
// deliberate differences. The template's diagrams are JavaScript functions an
// author writes; ours are data (see services/labs/schema/step.schema.json),
// which is what lets CI check them for overlap instead of asking a human to
// open the page and look. And `frame` is ours - the template has no way to draw
// a container, which turned out to be the first thing the first course needed.
//
// Coordinate space is the template's: a 640x300 viewBox, content inside
// x 20-620, y 30-270, with y 240-270 reserved for the caption.

export type Tone = "neutral" | "accent" | "warn" | "danger";

export type Shape =
  | { type: "box"; x: number; y: number; w: number; h: number; label: string; sub?: string; tone?: Tone; delay?: number }
  | { type: "frame"; x: number; y: number; w: number; h: number; label?: string; tone?: Tone; delay?: number }
  | { type: "wire"; from: Point; to: Point; tone?: Tone; dashed?: boolean; arrow?: boolean; delay?: number }
  | { type: "label"; x: number; y: number; text: string; anchor?: "start" | "middle" | "end"; tone?: Tone; delay?: number }
  | { type: "seal"; cx: number; cy: number; r?: number; broken?: boolean; delay?: number };

export type Point = { x: number; y: number };
export type Diagram = { caption?: string; shapes: Shape[] };

// Tones map to CSS variables rather than literal colours so the diagram follows
// the app's theme, and so "accent" stays a semantic choice an author makes
// rather than a colour they pick.
const stroke: Record<Tone, string> = {
  neutral: "var(--tp-hair, #2A3440)",
  accent: "var(--tp-accent, #49C97E)",
  warn: "var(--tp-warn, #E7B24A)",
  danger: "var(--tp-danger, #E0645C)",
};
const fill: Record<Tone, string> = {
  neutral: "rgba(148,163,184,.06)",
  accent: "rgba(73,201,126,.10)",
  warn: "rgba(231,178,74,.10)",
  danger: "rgba(224,100,92,.10)",
};
const ink: Record<Tone, string> = {
  neutral: "var(--tp-ink, #D6E2EE)",
  accent: "var(--tp-accent, #49C97E)",
  warn: "var(--tp-warn, #E7B24A)",
  danger: "var(--tp-danger, #E0645C)",
};
const MUTED = "var(--tp-muted, #8A9BAC)";

// Every value that reaches the SVG is escaped. Content is PR-reviewed rather
// than user-submitted, so this is defence in depth - but a renderer that
// concatenates strings into markup should escape regardless of who wrote them,
// because the next caller may not be so careful.
function esc(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}

const num = (n: number | undefined, fallback = 0) => (Number.isFinite(n) ? (n as number) : fallback);

function markers(): string {
  return (
    "<defs>" +
    (["neutral", "accent", "warn", "danger"] as Tone[])
      .map(
        (t) =>
          `<marker id="ar-${t}" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">` +
          `<path d="M0 0L10 5L0 10z" fill="${stroke[t]}"/></marker>`,
      )
      .join("") +
    "</defs>"
  );
}

function box(s: Extract<Shape, { type: "box" }>): string {
  const t = s.tone ?? "neutral";
  const cx = s.x + s.w / 2;
  const cy = s.y + s.h / 2;
  const labelY = s.sub ? cy - 4 : cy + 1;
  return (
    `<g class="cd-pop" style="animation-delay:${num(s.delay)}s">` +
    `<rect x="${s.x}" y="${s.y}" width="${s.w}" height="${s.h}" rx="10" fill="${fill[t]}" stroke="${stroke[t]}" stroke-width="1.5"/>` +
    `<text x="${cx}" y="${labelY}" text-anchor="middle" font-size="14" font-weight="500" fill="${ink[t]}">${esc(s.label)}</text>` +
    (s.sub
      ? `<text x="${cx}" y="${cy + 14}" text-anchor="middle" font-size="10.5" fill="${MUTED}">${esc(s.sub)}</text>`
      : "") +
    `</g>`
  );
}

// A frame's label sits ON the top border rather than inside, so it can never
// collide with the shapes the frame contains - which is the same reason the
// lint exempts frames from overlap checks.
function frame(s: Extract<Shape, { type: "frame" }>): string {
  const t = s.tone ?? "neutral";
  return (
    `<g class="cd-fade" style="animation-delay:${num(s.delay)}s">` +
    `<rect x="${s.x}" y="${s.y}" width="${s.w}" height="${s.h}" rx="12" fill="none" ` +
    `stroke="${stroke[t]}" stroke-width="1.2" stroke-dasharray="5 5" opacity=".65"/>` +
    (s.label
      ? `<text x="${s.x + 14}" y="${s.y + 4}" font-size="10.5" letter-spacing=".08em" fill="${MUTED}">${esc(s.label)}</text>`
      : "") +
    `</g>`
  );
}

function wire(s: Extract<Shape, { type: "wire" }>): string {
  const t = s.tone ?? "neutral";
  const arrow = s.arrow === false ? "" : ` marker-end="url(#ar-${t})"`;
  const dash = s.dashed ? ` stroke-dasharray="6 5"` : "";
  return (
    `<line class="cd-draw" style="animation-delay:${num(s.delay)}s" ` +
    `x1="${s.from.x}" y1="${s.from.y}" x2="${s.to.x}" y2="${s.to.y}" ` +
    `stroke="${stroke[t]}" stroke-width="1.8"${dash}${arrow}/>`
  );
}

function label(s: Extract<Shape, { type: "label" }>): string {
  const t = s.tone ?? "neutral";
  const colour = t === "neutral" ? MUTED : stroke[t];
  return (
    `<text class="cd-fade" style="animation-delay:${num(s.delay)}s" x="${s.x}" y="${s.y}" ` +
    `text-anchor="${s.anchor ?? "middle"}" font-size="11" fill="${colour}">${esc(s.text)}</text>`
  );
}

function seal(s: Extract<Shape, { type: "seal" }>): string {
  const r = num(s.r, 26);
  const broken = !!s.broken;
  const col = broken ? stroke.danger : stroke.accent;
  const bg = broken ? fill.danger : fill.accent;
  const glyph = broken
    ? `<path d="M${s.cx - 6} ${s.cy - 8} L${s.cx + 2} ${s.cy} L${s.cx - 5} ${s.cy + 2} L${s.cx + 6} ${s.cy + 9}" ` +
      `fill="none" stroke="${stroke.danger}" stroke-width="2.2" stroke-linecap="round"/>`
    : `<path d="M${s.cx - 6} ${s.cy} l4 4 l8 -8" fill="none" stroke="${stroke.accent}" stroke-width="2.4" ` +
      `stroke-linecap="round" stroke-linejoin="round"/>`;
  return (
    `<g class="cd-pop" style="animation-delay:${num(s.delay)}s">` +
    `<circle cx="${s.cx}" cy="${s.cy}" r="${r}" fill="${bg}" stroke="${col}" stroke-width="2"/>` +
    glyph +
    `</g>`
  );
}

// renderDiagram returns SVG markup. Frames are drawn first regardless of their
// position in the list, so a container can never paint over its own contents -
// authors shouldn't have to think about z-order in a format whose whole point
// is that the machine checks the layout.
export function renderDiagram(d: Diagram): string {
  const frames = d.shapes.filter((s) => s.type === "frame");
  const rest = d.shapes.filter((s) => s.type !== "frame");

  const body = [...frames, ...rest]
    .map((s) => {
      switch (s.type) {
        case "box":
          return box(s);
        case "frame":
          return frame(s);
        case "wire":
          return wire(s);
        case "label":
          return label(s);
        case "seal":
          return seal(s);
        default:
          return "";
      }
    })
    .join("");

  // The caption lives in the reserved band, drawn by the renderer rather than
  // positioned by the author - which is what makes "a label wandered into the
  // caption" a lint-able mistake instead of an inevitable one.
  const caption = d.caption
    ? `<text class="cd-fade" style="animation-delay:.9s" x="320" y="258" text-anchor="middle" ` +
      `font-size="11" fill="${MUTED}">${esc(d.caption)}</text>`
    : "";

  return `<svg viewBox="0 0 640 300" role="img" width="100%" height="100%">${markers()}${body}${caption}</svg>`;
}
