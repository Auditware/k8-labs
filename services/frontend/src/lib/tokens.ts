// Single source of truth for the palette that lives outside Tailwind's built-in
// slate/blue scale. Imported by tailwind.config.ts so these become utilities
// (bg-surface, border-hairline, text-status-danger, ...), and imported directly
// by the charts - whose bar fills and shade() ramp are computed at runtime and
// so cannot be class names. One definition feeds both, so the green in a chart
// bar and the green in a utility class can never drift apart.
export const palette = {
  surface: "#0b1120",
  hairline: "#1E2A36",
  hairlineStrong: "#222C34",
  accent: "#6AA6F2",
  ok: "#49C97E",
  warn: "#E7B24A",
  danger: "#F26D6D",
  // Neutral "reserved overhead" fill for the non-lab platform baseline in the
  // capacity columns - deliberately not a status colour, so it reads as context
  // (the node is not empty) rather than as a warning.
  system: "#475569",
  // The draggable quota-limit marker on the usage bar. A distinct orange so it
  // reads as an adjustable boundary, separate from the green/amber/red usage.
  limit: "#F97316",
} as const;
