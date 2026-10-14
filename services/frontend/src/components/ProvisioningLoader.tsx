import { useEffect, useRef } from "react";

// The provisioning screen shown while a lab's isolated cluster boots (~40s on prod:
// a Firecracker microVM boot then a foothold image pull, ~half the wait each; a
// cold image pull is what pushes it past that). Instead of a spinner it
// plays one of four hand-built canvas animations, picked at random per mount so
// repeat visits get variety.
//
// The progress bar prefers REAL backend progress: provisionNested reports each
// gate it clears (pod-ready, foothold-ready, ...) on the session poll, and we
// anchor the bar to that (see PHASE_ANCHOR). Anchoring to the real signal is what
// stops a slow, busy-hours boot from running the bar out ahead of reality. Until
// the first gate is reported - or on a legacy module that never reports one - the
// bar falls back to a time estimate (estimateSec) eased through the measured phase
// shape. Either way it holds shy of 100% while still provisioning; the parent
// swaps in the terminal the instant status flips to "running".

type Ctx = CanvasRenderingContext2D;

// ---- measured phase SHAPE (relative ms) → visual bands ----
// These weights only shape the time-based FALLBACK: what fraction of the wait each
// phase occupies. Absolute duration comes from estimateSec, not their sum. The ms
// are PROD p50 (from the backend's `provision phase` slog, harvested via Loki over
// 30d) - prod, not local k3d, is what students actually wait through, and the two
// differ sharply: on prod the real Firecracker microVM boot is ~18.6s (vs ~5.5s in
// a local k3d container) and the foothold image pull ~20s, so the wait is split
// roughly in half between those two gates (~39s total); everything else is sub-
// 200ms. band tracks that split so the fallback bar moves at a roughly even pace.
const PHASES = [
  { label: "Scheduling the runner", ms: 13, band: 0.05 },
  { label: "Booting your isolated microVM", ms: 18624, band: 0.44 },
  { label: "Assembling the nested cluster", ms: 80, band: 0.05 },
  { label: "Applying the lab", ms: 166, band: 0.06 },
  { label: "Bringing your foothold online", ms: 20074, band: 0.4 },
];
const TOTAL_MS = PHASES.reduce((a, p) => a + p.ms, 0);
const BANDS = (() => {
  let acc = 0,
    bacc = 0;
  return PHASES.map((p) => {
    const t0 = acc / TOTAL_MS;
    acc += p.ms;
    const b0 = bacc;
    bacc += p.band;
    return { ...p, t0, t1: acc / TOTAL_MS, b0, b1: bacc };
  });
})();
function progressFor(e: number) {
  e = Math.min(1, Math.max(0, e));
  let ph = BANDS[BANDS.length - 1];
  for (const p of BANDS) {
    if (e <= p.t1) {
      ph = p;
      break;
    }
  }
  const f = ph.t1 > ph.t0 ? (e - ph.t0) / (ph.t1 - ph.t0) : 1;
  return { label: ph.label, pct: ph.b0 + (ph.b1 - ph.b0) * Math.min(1, Math.max(0, f)) };
}

// ---- per-module click-to-running estimate (seconds) ----
// Used only by the time-based FALLBACK (bar + ETA) when no backend phase is in
// hand yet. Prod click->running measured at p50 ~39s (Loki, small n); this sits a
// little above that for tail headroom, since a bar that arrives slightly early
// reads better than one stuck at "almost there". Real phase progress, when
// present, overrides this entirely - so its exact value is low-stakes.
const DEFAULT_ESTIMATE_SEC = 42;
const ESTIMATE_SEC: Record<string, number> = {
  // An eBPF lab boots a heavier runner (2.5 GiB, extra kernel bits) and pulls a
  // larger foothold image, so it consistently runs past the default.
  "ebpf-detect": 55,
};
const estimateSec = (module?: string) => (module && ESTIMATE_SEC[module]) || DEFAULT_ESTIMATE_SEC;

// ---- backend milestone → where the bar sits ----
// provisionNested reports the gate it just cleared. floor is the progress that
// milestone has genuinely earned; while we wait for the next one the bar eases
// from floor toward ceil but never reaches it, so the next real milestone always
// visibly advances the bar rather than the clock faking the last stretch. pace is
// the measured PROD seconds the following gate typically takes - the two long ones
// (microVM boot ~18.6s and the foothold image pull ~20s) creep slowly, the sub-
// second gates snap. Floors track prod's cumulative time share (boot ≈ the first
// half, foothold ≈ the second), so the bar advances at a roughly even rate rather
// than stalling in whichever gate a fixed guess got wrong. label is the student-
// facing phrase for the work now in flight.
type Anchor = { floor: number; ceil: number; pace: number; label: string };
const PHASE_ANCHOR: Record<string, Anchor> = {
  "": { floor: 0.0, ceil: 0.05, pace: 1, label: "Scheduling the runner" },
  "pod-created": { floor: 0.05, ceil: 0.49, pace: 18, label: "Booting your isolated microVM" },
  "pod-ready": { floor: 0.49, ceil: 0.54, pace: 1, label: "Assembling the nested cluster" },
  "kubeconfig-fetched": { floor: 0.51, ceil: 0.54, pace: 1, label: "Assembling the nested cluster" },
  "nested-node-ready": { floor: 0.54, ceil: 0.6, pace: 1, label: "Applying the lab" },
  "manifests-applied": { floor: 0.6, ceil: 0.97, pace: 20, label: "Bringing your foothold online" },
  "foothold-ready": { floor: 0.97, ceil: 0.99, pace: 1, label: "Bringing your foothold online" },
};

// provisionHud is the whole "what does the bar/label/ETA say right now" decision,
// pulled out of the rAF draw so it's a pure function that can be unit-tested
// without a canvas (jsdom has none, which is why the draw loop can't be asserted
// directly). pct is 0..1 and always < 1 while provisioning; the caller caps the
// rendered bar at 96% so it never reads as "done" before status flips to running.
export function provisionHud(o: { reduce: boolean; phase: string | undefined; elapsedSec: number; phaseElapsedSec: number; estSec: number }): {
  pct: number;
  label: string;
  eta: string;
} {
  let pct: number, label: string;
  if (o.reduce) {
    // No animation frames: a single representative frame, no live countdown.
    const st = progressFor(0.6);
    return { pct: st.pct, label: st.label, eta: "" };
  } else if (o.phase !== undefined) {
    // Real backend milestone: sit at its earned floor, creep toward (never reach)
    // the next so the following real milestone always advances the bar.
    const a = PHASE_ANCHOR[o.phase] ?? PHASE_ANCHOR[""];
    const creep = 1 - Math.exp(-o.phaseElapsedSec / Math.max(0.5, a.pace));
    pct = a.floor + (a.ceil - a.floor) * creep;
    label = a.label;
  } else {
    // No milestone yet (or a legacy module): time estimate through the phase shape.
    const st = progressFor(Math.min(0.985, o.elapsedSec / o.estSec));
    pct = st.pct;
    label = st.label;
  }
  // Honest at both ends: bar essentially done (real foothold gate) -> say so;
  // otherwise count the estimate down; once past it, admit it's running long
  // rather than freezing on a stale "~Ns".
  let eta: string;
  if (pct >= 0.9) eta = "almost there…";
  else if (o.elapsedSec < o.estSec - 1) eta = "~" + Math.ceil(o.estSec - o.elapsedSec) + "s";
  else if (o.elapsedSec < o.estSec * 1.8) eta = "almost there…";
  else eta = "taking longer than usual…";
  return { pct, label, eta };
}

// ---- tiny canvas helpers ----
const c01 = (v: number) => Math.min(1, Math.max(0, v));
const ease = (v: number) => (v < 0.5 ? 2 * v * v : 1 - Math.pow(-2 * v + 2, 2) / 2);
const lerp = (a: number, b: number, t: number) => a + (b - a) * t;
function rr(c: Ctx, x: number, y: number, w: number, h: number, r: number) {
  c.beginPath();
  c.moveTo(x + r, y);
  c.arcTo(x + w, y, x + w, y + h, r);
  c.arcTo(x + w, y + h, x, y + h, r);
  c.arcTo(x, y + h, x, y, r);
  c.arcTo(x, y, x + w, y, r);
  c.closePath();
}
function hexPath(c: Ctx, cx: number, cy: number, r: number, rot = 0) {
  c.beginPath();
  for (let i = 0; i < 6; i++) {
    const a = rot + Math.PI / 6 + i * (Math.PI / 3);
    const x = cx + r * Math.cos(a),
      y = cy + r * Math.sin(a);
    i ? c.lineTo(x, y) : c.moveTo(x, y);
  }
  c.closePath();
}
function node(c: Ctx, x: number, y: number, s: number, ready: boolean, col: string) {
  rr(c, x - s, y - s * 0.72, s * 2, s * 1.44, 3.2);
  c.strokeStyle = ready ? "rgba(51,214,160,.9)" : col;
  c.lineWidth = 1.4;
  c.fillStyle = ready ? "rgba(51,214,160,.13)" : "rgba(120,100,220,.06)";
  c.fill();
  c.stroke();
  if (ready) {
    c.fillStyle = "#33d6a0";
    c.beginPath();
    c.arc(x, y, 1.7, 0, 7);
    c.fill();
  }
}
function tentacle(c: Ctx, bx: number, by: number, tx: number, ty: number, g: number, tt: number, i: number, col: string, wid: number) {
  const segs = 16,
    nrm = Math.atan2(ty - by, tx - bx) + Math.PI / 2,
    len = Math.hypot(tx - bx, ty - by);
  c.beginPath();
  for (let s = 0; s <= segs; s++) {
    const u = s / segs,
      px = bx + (tx - bx) * u,
      py = by + (ty - by) * u,
      wob = Math.sin(u * 7 - tt * 4 + i) * len * 0.1 * (1 - u) * g;
    c.lineTo(px + Math.cos(nrm) * wob, py + Math.sin(nrm) * wob);
  }
  c.strokeStyle = col;
  c.lineWidth = wid * Math.min(1, g) + 0.5;
  c.lineCap = "round";
  c.stroke();
}
function hexcells(cx: number, cy: number, s: number, R: number) {
  const out: { x: number; y: number; ring: number }[] = [];
  for (let q = -R; q <= R; q++)
    for (let r = -R; r <= R; r++) {
      const ring = (Math.abs(q) + Math.abs(r) + Math.abs(-q - r)) / 2;
      if (ring > R) continue;
      out.push({ x: cx + s * 1.5 * q, y: cy + s * Math.sqrt(3) * (r + q / 2), ring });
    }
  return out;
}

// ---- shared decorative state (persists across mounts; purely cosmetic) ----
const STARS: { x: number; y: number; s: number; tw: number }[] = [];
const BEES: { p: number; sp: number; rx: number; ry: number; ph: number }[] = [];

// ================= Animation 1 · Constellation =================
function constel(ctx: Ctx, w: number, h: number, e: number, tt: number) {
  const cx = w / 2,
    cy = h / 2,
    R = Math.min(w, h) * 0.36,
    N = 6;
  if (!STARS.length) for (let i = 0; i < 16; i++) STARS.push({ x: Math.random(), y: Math.random(), s: Math.random() * 0.4 + 0.15, tw: Math.random() * 7 });
  STARS.forEach((st) => {
    ctx.fillStyle = `rgba(150,170,230,${0.1 + 0.1 * Math.sin(tt * 1.5 + st.tw)})`;
    ctx.beginPath();
    ctx.arc(st.x * w, st.y * h, st.s, 0, 7);
    ctx.fill();
  });
  for (let i = 0; i < N; i++) {
    const a = -Math.PI / 2 + i * ((2 * Math.PI) / N),
      nx = cx + R * Math.cos(a),
      ny = cy + R * Math.sin(a),
      g = c01((e - (0.05 + i * 0.028)) / 0.13);
    if (g <= 0) continue;
    const lx = cx + (nx - cx) * ease(g),
      ly = cy + (ny - cy) * ease(g);
    ctx.strokeStyle = `rgba(79,125,255,${0.18 + 0.22 * g})`;
    ctx.lineWidth = 1.4;
    ctx.beginPath();
    ctx.moveTo(cx, cy);
    ctx.lineTo(lx, ly);
    ctx.stroke();
    if (g >= 1) {
      const pk = (tt * 0.5 + i * 0.16) % 1;
      ctx.fillStyle = "rgba(120,160,255,.9)";
      ctx.beginPath();
      ctx.arc(cx + (nx - cx) * pk, cy + (ny - cy) * pk, 1.6, 0, 7);
      ctx.fill();
    }
    node(ctx, nx, ny, 16 * Math.min(1, g), g >= 1, "rgba(79,125,255,.55)");
  }
  const cp = c01((e - 0.02) / 0.06),
    cs = Math.min(w, h) * 0.09;
  ctx.save();
  ctx.translate(cx, cy);
  ctx.scale(cp, cp);
  ctx.shadowColor = "rgba(79,125,255,.7)";
  ctx.shadowBlur = 18;
  rr(ctx, -cs, -cs * 0.8, cs * 2, cs * 1.6, 6);
  ctx.fillStyle = "#0e1830";
  ctx.fill();
  ctx.shadowBlur = 0;
  ctx.strokeStyle = "rgba(120,160,255,.9)";
  ctx.lineWidth = 1.6;
  ctx.stroke();
  ctx.strokeStyle = "rgba(150,185,255,.9)";
  ctx.lineWidth = 1.3;
  ctx.beginPath();
  ctx.arc(0, 0, cs * 0.4, 0, 7);
  ctx.stroke();
  for (let i = 0; i < 6; i++) {
    const a = i * (Math.PI / 3);
    ctx.beginPath();
    ctx.moveTo(cs * 0.4 * Math.cos(a), cs * 0.4 * Math.sin(a));
    ctx.lineTo(cs * 0.6 * Math.cos(a), cs * 0.6 * Math.sin(a));
    ctx.stroke();
  }
  ctx.restore();
  if (e > 0.34) {
    const g = c01((e - 0.34) / 0.6),
      a = -Math.PI / 2,
      nx = cx + R * Math.cos(a),
      ny = cy + R * Math.sin(a),
      px = cx + (nx - cx) * ease(g),
      py = cy + (ny - cy) * ease(g),
      done = g >= 1;
    ctx.shadowColor = done ? "rgba(51,214,160,.9)" : "rgba(120,160,255,.8)";
    ctx.shadowBlur = 14;
    ctx.fillStyle = done ? "#33d6a0" : "#7aa0ff";
    ctx.beginPath();
    ctx.arc(px, py, done ? 4 : 3.2, 0, 7);
    ctx.fill();
    ctx.shadowBlur = 0;
  }
}

// ================= Animation 2 · Deep-sea octopus (minimal-dots eyes) =================
function dotsEyes(ctx: Ctx, hr: number, blink: number) {
  [-1, 1].forEach((sx) => {
    const ex = sx * 0.3 * hr,
      ey = -0.03 * hr;
    ctx.fillStyle = "#180a33";
    ctx.beginPath();
    ctx.ellipse(ex, ey, 0.11 * hr, 0.12 * hr * blink, 0, 0, 7);
    ctx.fill();
    ctx.fillStyle = "rgba(255,255,255,.85)";
    ctx.beginPath();
    ctx.arc(ex - 0.035 * hr, ey - 0.04 * hr, 0.03 * hr * blink, 0, 7);
    ctx.fill();
  });
}
function deep(ctx: Ctx, w: number, h: number, e: number, tt: number) {
  const cx = w / 2,
    cy = h * 0.45,
    base = Math.min(w, h),
    R = base * 0.4,
    N = 8;
  ctx.strokeStyle = "rgba(154,120,255,.09)";
  ctx.lineWidth = 1;
  for (let i = 0; i < N; i++) {
    const a = -Math.PI / 2 + i * ((2 * Math.PI) / N);
    ctx.beginPath();
    ctx.moveTo(cx, cy);
    ctx.lineTo(cx + R * Math.cos(a), cy + R * Math.sin(a));
    ctx.stroke();
  }
  for (let i = 0; i < 12; i++) {
    const a = tt * 0.28 + i * 2.1,
      d = base * (0.14 + 0.3 * ((i % 5) / 5));
    ctx.fillStyle = "rgba(180,150,255,.32)";
    ctx.beginPath();
    ctx.arc(cx + Math.cos(a) * d, cy + Math.sin(a * 1.3) * d, 0.9, 0, 7);
    ctx.fill();
  }
  for (let i = 0; i < N; i++) {
    const a = -Math.PI / 2 + i * ((2 * Math.PI) / N),
      tx = cx + R * Math.cos(a),
      ty = cy + R * Math.sin(a),
      // The first tentacle reaches out faster than the rest (a shorter growth
      // span), so one hand visibly connects to its node early - without it, all
      // eight creep out together and for the first couple seconds it's unclear
      // what's happening.
      g = c01((e - (0.1 + i * 0.045)) / (i === 0 ? 0.16 : 0.3)),
      done = g >= 1,
      bx = cx + base * 0.06 * Math.cos(a),
      by = cy + base * 0.05 * Math.sin(a) + base * 0.04,
      tipx = bx + (tx - bx) * ease(g),
      tipy = by + (ty - by) * ease(g);
    ctx.shadowColor = "rgba(154,120,255,.5)";
    ctx.shadowBlur = 7;
    tentacle(ctx, bx, by, tipx, tipy, g, tt, i, `rgba(170,140,255,${0.3 + 0.5 * g})`, 2.8);
    ctx.shadowBlur = 0;
    if (g > 0) {
      const s = (6 + 3 * g) * 1.2; // node the tentacle grabs, +20% so it reads
      rr(ctx, tx - s, ty - s * 0.72, s * 2, s * 1.44, 3);
      ctx.strokeStyle = done ? "rgba(51,214,160,.9)" : "rgba(170,140,255,.75)";
      ctx.lineWidth = 1.3;
      ctx.fillStyle = done ? "rgba(51,214,160,.14)" : "rgba(154,120,255,.08)";
      ctx.fill();
      ctx.stroke();
      if (done) {
        ctx.fillStyle = "#33d6a0";
        ctx.beginPath();
        ctx.arc(tx, ty, 1.8, 0, 7);
        ctx.fill();
      }
    }
  }
  const hr = base * 0.1215, // head, -10% from 0.135 (was a touch too big)
    bob = Math.sin(tt * 1.8) * base * 0.008;
  ctx.save();
  ctx.translate(cx, cy + bob);
  ctx.shadowColor = "rgba(154,120,255,.6)";
  ctx.shadowBlur = 30;
  ctx.beginPath();
  ctx.ellipse(0, -hr * 0.18, hr, hr * 1.06, 0, 0, 7);
  const g = ctx.createLinearGradient(0, -hr, 0, hr);
  g.addColorStop(0, "#c9b4ff");
  g.addColorStop(0.55, "#9a78ff");
  g.addColorStop(1, "#6b47d6");
  ctx.fillStyle = g;
  ctx.fill();
  ctx.shadowBlur = 0;
  ctx.strokeStyle = "rgba(210,192,255,.35)";
  ctx.lineWidth = 1;
  ctx.beginPath();
  ctx.ellipse(0, -hr * 0.18, hr, hr * 1.06, 0, 0, 7);
  ctx.stroke();
  const blink = Math.sin(tt * 1.15) > 0.975 ? 0.16 : 1;
  dotsEyes(ctx, hr, blink);
  ctx.restore();
}

// ================= Animation 3 · Honeycomb (bees) =================
function beeSprite(ctx: Ctx, x: number, y: number, ang: number, tt: number, i: number) {
  ctx.save();
  ctx.translate(x, y);
  ctx.rotate(ang);
  const wf = 0.45 + 0.55 * Math.abs(Math.sin(tt * 34 + i));
  ctx.fillStyle = "rgba(225,235,255," + (0.35 + 0.35 * wf) + ")";
  ctx.beginPath();
  ctx.ellipse(0.4, -1.9, 1.9, 0.8 + 1.1 * wf, -0.5, 0, 7);
  ctx.fill();
  ctx.beginPath();
  ctx.ellipse(0.4, 1.9, 1.9, 0.8 + 1.1 * wf, 0.5, 0, 7);
  ctx.fill();
  ctx.fillStyle = "#f6c33f";
  ctx.beginPath();
  ctx.ellipse(0, 0, 3.4, 2.1, 0, 0, 7);
  ctx.fill();
  ctx.strokeStyle = "rgba(40,26,6,.85)";
  ctx.lineWidth = 0.9;
  ctx.beginPath();
  ctx.moveTo(-0.8, -1.9);
  ctx.lineTo(-0.8, 1.9);
  ctx.moveTo(0.9, -1.7);
  ctx.lineTo(0.9, 1.7);
  ctx.stroke();
  ctx.restore();
}
function bees(ctx: Ctx, w: number, h: number, e: number, tt: number) {
  const s = Math.min(w, h) * 0.085,
    cx = w / 2,
    cy = h / 2,
    R = 2;
  const cells = hexcells(cx, cy, s, R);
  cells.forEach((cc, i) => {
    const g = c01((e - cc.ring * 0.24) / 0.34);
    hexPath(ctx, cc.x, cc.y, s * 0.9, 0);
    ctx.strokeStyle = `rgba(244,166,58,${0.14 + 0.35 * g})`;
    ctx.lineWidth = 1.3;
    ctx.stroke();
    if (g > 0) {
      ctx.fillStyle = `rgba(244,166,58,${0.1 + 0.5 * g})`;
      if (g >= 1) {
        ctx.shadowColor = "rgba(244,166,58,.55)";
        ctx.shadowBlur = 10;
      }
      hexPath(ctx, cc.x, cc.y, s * 0.9 * (0.5 + 0.5 * g), 0);
      ctx.fill();
      ctx.shadowBlur = 0;
      if (g >= 1) {
        ctx.fillStyle = `rgba(255,214,140,${0.25 * Math.max(0, Math.sin(tt * 3 + i))})`;
        hexPath(ctx, cc.x, cc.y, s * 0.5, 0);
        ctx.fill();
      }
    }
  });
  if (!BEES.length) for (let i = 0; i < 6; i++) BEES.push({ p: Math.random() * 7, sp: 0.5 + Math.random() * 0.5, rx: 0.24 + Math.random() * 0.14, ry: 0.18 + Math.random() * 0.12, ph: Math.random() * 7 });
  BEES.forEach((b, i) => {
    const a = tt * b.sp + b.p,
      jitter = Math.sin(tt * 9 + i) * 0.05,
      rx = Math.min(w, h) * b.rx,
      ry = Math.min(w, h) * b.ry;
    const x = cx + Math.cos(a + jitter) * rx * (1 + 0.12 * Math.sin(tt * 2 + i)),
      y = cy + Math.sin(a * 1.15 + b.ph) * ry;
    const nx = cx + Math.cos(a + 0.1) * rx,
      ny = cy + Math.sin((a + 0.1) * 1.15 + b.ph) * ry;
    beeSprite(ctx, x, y, Math.atan2(ny - y, nx - x), tt, i);
  });
}

// ================= Animation 4 · microVM vault + checklist =================
const VLIST = [
  { l: "NetworkPolicy", on: 0.1 },
  { l: "Firecracker microVM", on: 0.52 },
  { l: "nested k3s", on: 0.7 },
  { l: "your foothold", on: 0.95 },
];
function vault(ctx: Ctx, w: number, h: number, e: number, tt: number) {
  const base = Math.min(w, h),
    // Cap the hexagon by width too, so in a narrow/tall pane it can't grow into
    // the checklist to its right.
    R = Math.min(base * 0.3, w * 0.24),
    cx = w * 0.36,
    cy = h / 2;
  const seal = c01((e - 0.02) / 0.5);
  ctx.lineCap = "round";
  for (let i = 0; i < 6; i++) {
    const a0 = Math.PI / 6 + i * (Math.PI / 3),
      a1 = a0 + Math.PI / 3,
      edge = c01(seal * 6 - i);
    if (edge <= 0) continue;
    const x0 = cx + R * Math.cos(a0),
      y0 = cy + R * Math.sin(a0),
      x1 = cx + R * Math.cos(a1),
      y1 = cy + R * Math.sin(a1);
    ctx.strokeStyle = "rgba(110,150,255,.9)";
    ctx.shadowColor = "rgba(79,125,255,.5)";
    ctx.shadowBlur = 8;
    ctx.lineWidth = 2.4;
    ctx.beginPath();
    ctx.moveTo(x0, y0);
    ctx.lineTo(lerp(x0, x1, edge), lerp(y0, y1, edge));
    ctx.stroke();
    ctx.shadowBlur = 0;
  }
  const lock = c01((e - 0.3) / 0.4);
  ctx.save();
  ctx.translate(cx, cy);
  ctx.rotate(tt * (lock < 1 ? 2.2 : 0.4));
  ctx.strokeStyle = `rgba(120,170,255,${0.4 + 0.4 * lock})`;
  ctx.lineWidth = 2;
  ctx.setLineDash([base * 0.05, base * 0.05]);
  ctx.beginPath();
  ctx.arc(0, 0, R * 0.62, 0, 7);
  ctx.stroke();
  ctx.setLineDash([]);
  for (let i = 0; i < 3; i++) {
    const a = i * ((2 * Math.PI) / 3);
    ctx.fillStyle = "#9dc0ff";
    ctx.beginPath();
    ctx.arc(R * 0.62 * Math.cos(a), R * 0.62 * Math.sin(a), 2.4, 0, 7);
    ctx.fill();
  }
  ctx.restore();
  const cg = c01((e - 0.55) / 0.45),
    cr = base * 0.12;
  hexPath(ctx, cx, cy, cr, Math.PI / 6);
  ctx.fillStyle = `rgba(51,214,160,${0.1 + 0.5 * cg})`;
  ctx.shadowColor = "rgba(51,214,160," + 0.7 * cg + ")";
  ctx.shadowBlur = 6 + 24 * cg;
  ctx.fill();
  ctx.shadowBlur = 0;
  ctx.strokeStyle = `rgba(90,235,180,${0.4 + 0.5 * cg})`;
  ctx.lineWidth = 1.5;
  ctx.stroke();
  ctx.textBaseline = "middle";
  ctx.font = "500 11px ui-monospace,monospace";
  // Start the checklist just right of the hexagon's rightmost point, so the
  // vault outline never crosses the labels regardless of pane aspect.
  const lx = cx + R * 0.92 + w * 0.05;
  // Tight cluster centered on the hexagon rather than spread over the pane.
  const gap = h * 0.075;
  let iy = cy - gap * (VLIST.length - 1) * 0.5;
  VLIST.forEach((it) => {
    const on = e >= it.on;
    ctx.fillStyle = on ? "#cfeee1" : "rgba(140,155,190,.5)";
    ctx.fillText((on ? "● " : "○ ") + it.l, lx, iy);
    iy += gap;
  });
}

const FX = [constel, deep, bees, vault];

// The parent re-renders on every status poll while provisioning, which can
// remount us. Keying the animation choice AND the clock off the stable session
// id keeps both steady across those remounts (one steady animation per boot),
// while still varying from one session to the next. Without a seed we fall back
// to random per mount.
const START_TIMES = new Map<string, number>();
function hashIdx(s: string, n: number) {
  let h = 0;
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) >>> 0;
  return h % n;
}

export default function ProvisioningLoader({ variant, seed, phase, module }: { variant?: number; seed?: string; phase?: string; module?: string }) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const phaseRef = useRef<HTMLSpanElement>(null);
  const etaRef = useRef<HTMLSpanElement>(null);
  const barRef = useRef<HTMLDivElement>(null);
  // The rAF loop below runs off a mount-only effect, so it can't read the phase
  // prop directly (it would capture the first value forever). Mirror the latest
  // phase - and when it changed, so the bar's within-phase creep is measured from
  // the real transition, not from mount.
  const curPhase = useRef<string | undefined>(phase);
  const phaseAt = useRef<number>(0);
  useEffect(() => {
    if (curPhase.current !== phase) {
      curPhase.current = phase;
      phaseAt.current = performance.now();
    }
  }, [phase]);
  // Which animation: forced (tests/preview), else derived from the session id so
  // it's stable across remounts and varies per boot, else random.
  const fx = useRef(
    FX[variant != null ? ((variant % FX.length) + FX.length) % FX.length : seed ? hashIdx(seed, FX.length) : Math.floor(Math.random() * FX.length)],
  );

  useEffect(() => {
    const est = estimateSec(module);
    const cv = canvasRef.current;
    if (!cv) return;
    const reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    // Continuous clock across remounts: reuse the session's first-seen start.
    let start = performance.now();
    if (seed) {
      const prev = START_TIMES.get(seed);
      if (prev != null) start = prev;
      else START_TIMES.set(seed, start);
    }
    let raf = 0;

    const draw = (now: number) => {
      const dpr = Math.min(2, window.devicePixelRatio || 1);
      const w = cv.clientWidth,
        h = cv.clientHeight;
      if (cv.width !== Math.round(w * dpr) || cv.height !== Math.round(h * dpr)) {
        cv.width = Math.round(w * dpr);
        cv.height = Math.round(h * dpr);
      }
      const ctx = cv.getContext("2d");
      if (!ctx) return;
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      ctx.clearRect(0, 0, w, h);

      const elapsed = (now - start) / 1000;
      const { pct, label, eta } = provisionHud({
        reduce,
        phase: curPhase.current,
        elapsedSec: elapsed,
        phaseElapsedSec: (now - (phaseAt.current || start)) / 1000,
        estSec: est,
      });
      // Scene assembly tracks the same progress, "pre-warmed" so the canvas starts
      // partly built instead of fading up from empty (which read like a failure).
      const e = reduce ? 0.6 : Math.min(0.985, pct);
      const eDraw = reduce ? 0.6 : 0.2 + 0.8 * e;
      // Render the scene at 80% (centered) so it sits a little smaller in the pane.
      ctx.translate(w / 2, h / 2);
      ctx.scale(0.8, 0.8);
      ctx.translate(-w / 2, -h / 2);
      fx.current(ctx, w, h, eDraw, now / 1000);

      if (phaseRef.current) phaseRef.current.textContent = label + "…";
      if (etaRef.current && eta) etaRef.current.textContent = eta;
      if (barRef.current) barRef.current.style.width = Math.min(96, pct * 100).toFixed(1) + "%";

      if (!reduce) raf = requestAnimationFrame(draw);
    };
    raf = requestAnimationFrame(draw);
    return () => cancelAnimationFrame(raf);
  }, []);

  return (
    <div
      className="relative h-full w-full overflow-hidden bg-[#05070e]"
      style={{
        backgroundImage: "radial-gradient(rgba(120,140,190,0.10) 1px, transparent 1px)",
        backgroundSize: "16px 16px",
        backgroundPosition: "center",
      }}
    >
      <canvas ref={canvasRef} className="absolute inset-0 h-full w-full" aria-hidden="true" />
      <div className="pointer-events-none absolute inset-x-0 bottom-0 bg-gradient-to-t from-[#04070e]/90 to-transparent px-5 pb-4 pt-8">
        <div className="mx-auto flex max-w-md items-baseline justify-between gap-3">
          <span ref={phaseRef} className="font-mono text-xs text-slate-300">
            Provisioning your isolated cluster…
          </span>
          <span ref={etaRef} className="font-mono text-[11px] tabular-nums text-slate-500" />
        </div>
        <div className="mx-auto mt-2 h-1 max-w-md overflow-hidden rounded-full bg-slate-800/80">
          <div ref={barRef} className="h-full rounded-full bg-gradient-to-r from-blue-600 to-blue-400 shadow-[0_0_12px_rgba(79,125,255,.55)] transition-[width] duration-200 ease-linear" />
        </div>
      </div>
      <span className="sr-only" role="status">
        Provisioning your isolated cluster. This usually takes about half a minute.
      </span>
    </div>
  );
}
