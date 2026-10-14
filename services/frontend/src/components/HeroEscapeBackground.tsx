import { useEffect, useRef, type RefObject } from "react";

// Ambient hero background: a quiet field of dots that, at random spots and slow
// intervals, connects a few dots into a Kubernetes/cloud escape "hint" (lateral
// movement, namespace escape, pod->node breakout, cross-node lateral, privilege
// escalation, cloud-metadata theft, cluster fan-out, or the k8s wheel). Blue is
// benign movement; a dot flashes RED at the instant of compromise/elevation. A
// magnifying "eBPF sensor" lens glides over the newest red event and enlarges it
// - the Defender-lab story: attacks happen, and you watch them get caught.
//
// Rendered on a <canvas> from bundled JS (no inline script, same-origin), so the
// enforced prod CSP is unaffected. pointer-events:none, so it never intercepts
// clicks or scroll. Animation pauses when off-screen or the tab is hidden, and
// under prefers-reduced-motion it paints a single static frame and stops.

type Pt = { x: number; y: number; i?: number };
type Seg = [Pt, Pt, string?];
type Scene = {
  faint?: [Pt, Pt][];
  segs?: Seg[];
  arrows?: [Pt, Pt, string][];
  nodes?: [Pt, string][];
  lit?: Pt[];
  hot?: Pt;
  hotSet?: Pt[];
  path?: Pt[];
  alwaysHot?: boolean;
  burst?: Pt;
};
type Inst = { scene: Scene; t0: number; dur: number; a: number; pt: number };

const ACC = "#6aa6f2";
const ACCB = "#9cc2ff";
const RED = "#ff5a63";
const REDB = "#ff8a90";
const SPACING = 30;
const KEYS = [
  "lateral",
  "namespace",
  "podnode",
  "podother",
  "escalate",
  "metadata",
  "fanout",
  "k8s",
] as const;

export function HeroEscapeBackground({
  className,
  sensorTargetRef,
  onSensorAlert,
}: {
  className?: string;
  // Optional detectable target for the roaming lens (the landing wires the
  // rank-1 operator card here). When the lens passes over it the lens flips to
  // its alert look and onSensorAlert(true) fires so the DOM side can light up
  // too; onSensorAlert(false) on leaving. Purely cosmetic - a wink at the
  // sensor "catching" the top operator.
  sensorTargetRef?: RefObject<HTMLElement | null>;
  onSensorAlert?: (active: boolean) => void;
}) {
  const ref = useRef<HTMLCanvasElement | null>(null);
  // The animation closure below is set up once; these refs keep the latest
  // callback/target reachable from inside it.
  const onAlertRef = useRef(onSensorAlert);
  onAlertRef.current = onSensorAlert;
  const targetRef = useRef(sensorTargetRef);
  targetRef.current = sensorTargetRef;

  useEffect(() => {
    const canvas = ref.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    const reduce = window.matchMedia?.("(prefers-reduced-motion: reduce)")?.matches ?? false;
    const ri = (a: number, b: number) => (b < a ? a : a + Math.floor(Math.random() * (b - a + 1)));

    let w = 0;
    let h = 0;
    let cols = 0;
    let rows = 0;
    let flat: Pt[] = [];
    let tw: number[] = [];
    let ph: number[] = [];
    let LR = 70;
    let lx: number | null = null;
    let ly: number | null = null;
    const MAG = 1.7;
    // Canvas origin in viewport coords (it's a fixed full-screen layer, so this
    // is ~0,0, but read it from fit() to stay correct). Used to compare the lens
    // center against a target element's getBoundingClientRect.
    let cvLeft = 0;
    let cvTop = 0;

    const at = (c: number, r: number): Pt => {
      c = Math.max(0, Math.min(cols - 1, c));
      r = Math.max(0, Math.min(rows - 1, r));
      return flat[r * cols + c];
    };
    const lenOf = (p: Pt[]) => {
      let L = 0;
      for (let i = 1; i < p.length; i++) L += Math.hypot(p[i].x - p[i - 1].x, p[i].y - p[i - 1].y);
      return L;
    };
    const at01 = (p: Pt[], t: number): Pt => {
      let d = t * lenOf(p);
      for (let i = 1; i < p.length; i++) {
        const s = Math.hypot(p[i].x - p[i - 1].x, p[i].y - p[i - 1].y);
        if (d <= s) {
          const f = s ? d / s : 0;
          return { x: p[i - 1].x + (p[i].x - p[i - 1].x) * f, y: p[i - 1].y + (p[i].y - p[i - 1].y) * f };
        }
        d -= s;
      }
      return p[p.length - 1];
    };
    const head = (a: Pt, b: Pt, sz: number, al: number, col: string) => {
      const an = Math.atan2(b.y - a.y, b.x - a.x);
      ctx.save();
      ctx.globalAlpha = al;
      ctx.fillStyle = col;
      ctx.beginPath();
      ctx.moveTo(b.x, b.y);
      ctx.lineTo(b.x - sz * Math.cos(an - 0.42), b.y - sz * Math.sin(an - 0.42));
      ctx.lineTo(b.x - sz * Math.cos(an + 0.42), b.y - sz * Math.sin(an + 0.42));
      ctx.closePath();
      ctx.fill();
      ctx.restore();
    };
    const box = (c0: number, r0: number, bw: number, bh: number): [Pt, Pt][] => {
      const f: [Pt, Pt][] = [];
      for (let c = c0; c < c0 + bw; c++) {
        f.push([at(c, r0), at(c + 1, r0)]);
        f.push([at(c, r0 + bh), at(c + 1, r0 + bh)]);
      }
      for (let r = r0; r < r0 + bh; r++) {
        f.push([at(c0, r), at(c0, r + 1)]);
        f.push([at(c0 + bw, r), at(c0 + bw, r + 1)]);
      }
      return f;
    };

    const BUILD: Record<string, () => Scene> = {
      lateral() {
        const r0 = ri(1, rows - 2);
        const c0 = ri(1, cols - 8);
        const nn = ri(4, 6);
        let c = c0;
        let r = r0;
        const path = [at(c, r)];
        for (let i = 0; i < nn; i++) {
          c++;
          r += ri(-1, 1);
          path.push(at(c, r));
        }
        const segs: Seg[] = [];
        for (let i = 1; i < path.length - 1; i++) segs.push([path[i - 1], path[i]]);
        return { segs, arrows: [[path[path.length - 2], path[path.length - 1], ACC]], lit: path, path };
      },
      namespace() {
        const bw = 4;
        const bh = 3;
        const c0 = ri(1, cols - bw - 3);
        const r0 = ri(1, rows - bh - 1);
        const start = at(c0 + 1, r0 + 1);
        const edge = at(c0 + bw, r0 + 1);
        const out = at(c0 + bw + 2, r0 + 1);
        return {
          faint: box(c0, r0, bw, bh),
          segs: [[start, edge], [edge, out, "hot"]],
          arrows: [[edge, out, RED]],
          lit: [start, edge],
          path: [start, edge, out],
          hot: out,
          burst: out,
        };
      },
      podnode() {
        const c0 = ri(1, cols - 7);
        const r0 = ri(1, rows - 6);
        const nb = box(c0, r0, 5, 4);
        const pb = box(c0 + 1, r0 + 1, 2, 2);
        const pod = at(c0 + 2, r0 + 2);
        const pe = at(c0 + 3, r0 + 2);
        const host = at(c0 + 4, r0 + 3);
        return {
          faint: nb.concat(pb),
          segs: [[pod, pe], [pe, host, "hot"]],
          arrows: [[pe, host, RED]],
          lit: [pod, pe],
          path: [pod, pe, host],
          hot: host,
          burst: host,
        };
      },
      podother() {
        const r0 = ri(1, rows - 4);
        const c0 = ri(1, cols - 9);
        const A = box(c0, r0, 2, 2);
        const B = box(c0 + 5, r0, 2, 2);
        const pod = at(c0 + 1, r0 + 1);
        const ae = at(c0 + 2, r0 + 1);
        const dest = at(c0 + 6, r0 + 1);
        return {
          faint: A.concat(B),
          segs: [[pod, ae], [ae, dest, "hot"]],
          arrows: [[ae, dest, RED]],
          lit: [pod, ae],
          path: [pod, ae, dest],
          hot: dest,
          burst: dest,
        };
      },
      escalate() {
        const c = ri(2, cols - 2);
        const sr = ri(7, rows - 2);
        const steps = Math.min(5, sr - 1);
        let cc = c;
        let r = sr;
        const path = [at(cc, r)];
        for (let i = 0; i < steps; i++) {
          r--;
          cc += ri(-1, 1);
          path.push(at(cc, r));
        }
        const segs: Seg[] = [];
        for (let i = 1; i < path.length - 1; i++) segs.push([path[i - 1], path[i]]);
        const top = path[path.length - 1];
        return { segs, arrows: [[path[path.length - 2], top, RED]], lit: path.slice(0, -1), path, hot: top, burst: top };
      },
      metadata() {
        const ac = ri(1, cols - 6);
        const ar = ri(1, rows - 4);
        const pod = at(ac, ar + 2);
        const mid = at(ac + 2, ar + 1);
        const cloud = at(ac + 4, ar);
        return {
          segs: [[pod, mid], [mid, cloud, "hot"]],
          arrows: [[mid, cloud, RED]],
          lit: [pod, mid],
          nodes: [[cloud, RED]],
          path: [pod, mid, cloud],
          hot: cloud,
          burst: cloud,
        };
      },
      fanout() {
        const c = ri(3, cols - 4);
        const r = ri(2, rows - 3);
        const o = at(c, r);
        const dirs: [number, number][] = [
          [3, 0],
          [-3, 1],
          [1, -2],
          [2, 2],
          [-2, -1],
        ].slice(0, ri(3, 4)) as [number, number][];
        const tips = dirs.map((d) => at(c + d[0], r + d[1]));
        const segs: Seg[] = tips.map((t) => [o, t]);
        const path: Pt[] = [];
        tips.forEach((t) => {
          path.push(o);
          path.push(t);
        });
        return { segs, lit: [], hotSet: [o].concat(tips), path, alwaysHot: true };
      },
      k8s() {
        const cc = ri(3, cols - 4);
        const cr = ri(3, rows - 4);
        const C = at(cc, cr);
        const R = SPACING * 2.0;
        const rot = Math.random() * 6.28;
        const V: Pt[] = [];
        for (let i = 0; i < 7; i++) {
          const a = rot - Math.PI / 2 + (i * 2 * Math.PI) / 7;
          V.push({ x: C.x + Math.cos(a) * R, y: C.y + Math.sin(a) * R });
        }
        const segs: Seg[] = [];
        for (let i = 0; i < 7; i++) {
          segs.push([C, V[i]]);
          segs.push([V[i], V[(i + 1) % 7]]);
        }
        return { segs, lit: [C].concat(V), path: V.concat([V[0]]) };
      },
    };
    const hotOf = (s: Scene): Pt | null => s.hot || (s.hotSet && s.hotSet[s.hotSet.length - 1]) || null;

    const fit = () => {
      const rc = canvas.getBoundingClientRect();
      const dpr = Math.min(2, window.devicePixelRatio || 1);
      cvLeft = rc.left;
      cvTop = rc.top;
      w = rc.width;
      h = rc.height;
      canvas.width = Math.max(1, Math.round(rc.width * dpr));
      canvas.height = Math.max(1, Math.round(rc.height * dpr));
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      cols = Math.max(6, Math.floor(w / SPACING));
      rows = Math.max(4, Math.floor(h / SPACING));
      const ox = (w - (cols - 1) * SPACING) / 2;
      const oy = (h - (rows - 1) * SPACING) / 2;
      flat = [];
      tw = [];
      ph = [];
      let i = 0;
      for (let r = 0; r < rows; r++)
        for (let c = 0; c < cols; c++) {
          flat.push({ x: ox + c * SPACING, y: oy + r * SPACING, i });
          tw.push(Math.random() < 0.1 ? 1 : 0);
          ph.push(Math.random() * 6.28);
          i++;
        }
      LR = Math.max(52, Math.min(88, Math.min(w, h) * 0.13));
      if (lx == null) {
        lx = w * 0.5;
        ly = h * 0.55;
      }
    };

    // Cheap refresh of just the canvas origin (left/top move on scroll). Called
    // at most once per frame via the rectDirty flag, never from pointermove - see
    // onMove for why a per-move getBoundingClientRect is the wrong place for it.
    const syncRect = () => {
      const rc = canvas.getBoundingClientRect();
      cvLeft = rc.left;
      cvTop = rc.top;
    };

    const drawDots = (now: number) => {
      for (const p of flat) {
        let a = 0.14;
        if (tw[p.i as number]) a += 0.1 * (0.5 + 0.5 * Math.sin(now / 1050 + ph[p.i as number]));
        ctx.globalAlpha = 1;
        ctx.fillStyle = "rgba(150,168,190," + a.toFixed(3) + ")";
        ctx.beginPath();
        ctx.arc(p.x, p.y, 1.2, 0, 7);
        ctx.fill();
      }
    };

    const drawScene = (s: Scene, a: number, pt: number) => {
      ctx.lineCap = "round";
      (s.faint || []).forEach((g) => {
        ctx.globalAlpha = a * 0.18;
        ctx.strokeStyle = ACC;
        ctx.lineWidth = 1;
        ctx.beginPath();
        ctx.moveTo(g[0].x, g[0].y);
        ctx.lineTo(g[1].x, g[1].y);
        ctx.stroke();
      });
      ctx.shadowColor = ACC;
      ctx.shadowBlur = 8 * a;
      (s.segs || []).forEach((g) => {
        const hot = g[2] === "hot";
        ctx.globalAlpha = a * (g[2] ? 0.9 : 0.6);
        ctx.strokeStyle = hot ? RED : g[2] ? ACCB : ACC;
        ctx.shadowColor = hot ? RED : ACC;
        ctx.lineWidth = g[2] ? 1.6 : 1.1;
        ctx.beginPath();
        ctx.moveTo(g[0].x, g[0].y);
        ctx.lineTo(g[1].x, g[1].y);
        ctx.stroke();
      });
      ctx.shadowBlur = 0;
      (s.arrows || []).forEach((g) => {
        const col = g[2] || ACC;
        ctx.globalAlpha = a * 0.8;
        ctx.strokeStyle = col;
        ctx.lineWidth = 1.2;
        ctx.beginPath();
        ctx.moveTo(g[0].x, g[0].y);
        ctx.lineTo(g[1].x, g[1].y);
        ctx.stroke();
        head(g[0], g[1], 7, a * 0.9, col);
      });
      (s.nodes || []).forEach((nn) => {
        const p = nn[0];
        ctx.globalAlpha = a * 0.8;
        ctx.strokeStyle = nn[1] || ACC;
        ctx.lineWidth = 1.3;
        ctx.beginPath();
        ctx.arc(p.x, p.y, 7, 0, 7);
        ctx.stroke();
      });
      (s.lit || []).forEach((p) => {
        ctx.globalAlpha = a;
        ctx.fillStyle = ACCB;
        ctx.shadowColor = ACC;
        ctx.shadowBlur = 9 * a;
        ctx.beginPath();
        ctx.arc(p.x, p.y, 2.1, 0, 7);
        ctx.fill();
      });
      ctx.shadowBlur = 0;
      const on = s.alwaysHot ? 1 : Math.max(0, Math.min(1, (pt - 0.55) / 0.3));
      const hi = (s.hotSet || []).concat(s.hot ? [s.hot] : []);
      hi.forEach((p) => {
        ctx.globalAlpha = a * (0.1 + 0.9 * on);
        ctx.fillStyle = REDB;
        ctx.shadowColor = RED;
        ctx.shadowBlur = 13 * a;
        ctx.beginPath();
        ctx.arc(p.x, p.y, 2.7, 0, 7);
        ctx.fill();
      });
      ctx.shadowBlur = 0;
      if (s.path && s.path.length > 1) {
        const q = at01(s.path, pt);
        ctx.globalAlpha = a;
        ctx.fillStyle = "#eaf3ff";
        ctx.shadowColor = ACCB;
        ctx.shadowBlur = 12 * a;
        ctx.beginPath();
        ctx.arc(q.x, q.y, 2.5, 0, 7);
        ctx.fill();
        ctx.shadowBlur = 0;
      }
      ctx.globalAlpha = 1;
    };

    const envA = (e: number, dur: number) => {
      const fi = 650;
      const fo = 1200;
      return e < fi ? e / fi : e > dur - fo ? Math.max(0, (dur - e) / fo) : 1;
    };

    const inst: Inst[] = [];
    const MAX = 3;
    let nextSpawn = 0;
    let mx = 0;
    let my = 0;
    let manualUntil = 0;
    let prevNow = 0;
    let rectDirty = false;
    let inspecting = false;
    // Sensor-alert state: is the lens currently over the wired target element?
    // `alerting` drives the lens's own alert look; `lastAlert` gates the DOM
    // callback so it only fires on enter/leave; `sensorTick` throttles the
    // getBoundingClientRect check to a few times a second.
    let alerting = false;
    let lastAlert = false;
    let sensorTick = 0;

    const durFor = (k: string) => (k === "fanout" ? 5200 : k === "k8s" ? 4200 : 4700);
    const spawn = (now: number) => {
      if (inst.length >= MAX) return;
      const k = KEYS[ri(0, KEYS.length - 1)];
      inst.push({ scene: BUILD[k](), t0: now, dur: durFor(k), a: 0, pt: 0 });
    };

    const drawWorld = (now: number) => {
      drawDots(now);
      inst.forEach((o) => drawScene(o.scene, o.a, o.pt));
    };

    const updLens = (now: number) => {
      let tx = lx as number;
      let ty = ly as number;
      let following = false;
      if (now < manualUntil) {
        tx = mx;
        ty = my;
        following = true;
      } else {
        for (let i = inst.length - 1; i >= 0; i--) {
          const hh = hotOf(inst[i].scene);
          if (hh && inst[i].a > 0.25) {
            tx = hh.x;
            ty = hh.y;
            break;
          }
        }
      }
      // Frame-rate-independent smoothing so the follow feel doesn't slow down
      // when the scene gets heavy and FPS dips. When the pointer is driving the
      // lens we chase it tightly (near-instant, seamless); the ambient drift
      // between hot targets stays slow and glide-y.
      const dt = prevNow ? Math.min(64, now - prevNow) : 16;
      prevNow = now;
      const rate = following ? 170 : 4.5; // per second
      const s = 1 - Math.exp((-rate * dt) / 1000);
      lx = (lx as number) + (tx - (lx as number)) * s;
      ly = (ly as number) + (ty - (ly as number)) * s;
      // While the pointer is driving, collapse the last sub-pixel so the lens
      // rests exactly under the cursor - no lingering easing tail or shimmer. The
      // snap only fires once already within 0.6px, so re-acquiring the pointer
      // after an ambient drift still eases in smoothly rather than teleporting.
      if (
        following &&
        Math.abs(tx - (lx as number)) < 0.6 &&
        Math.abs(ty - (ly as number)) < 0.6
      ) {
        lx = tx;
        ly = ty;
      }
      lx = Math.max(LR, Math.min(w - LR, lx));
      ly = Math.max(LR, Math.min(h - LR, ly));
      inspecting = false;
      for (const o of inst) {
        const hh = hotOf(o.scene);
        if (hh && Math.hypot(hh.x - (lx as number), hh.y - (ly as number)) < LR * 0.75 && o.pt > 0.5) {
          inspecting = true;
          break;
        }
      }

      // Does the lens sit over the wired target (the rank-1 operator card)?
      // Throttled, and with a little hysteresis (a wider box once alerting) so a
      // lens hovering the edge doesn't flicker the alert on and off.
      if ((sensorTick++ & 3) === 0) {
        const tgt = targetRef.current?.current;
        let hit = false;
        if (tgt) {
          const tr = tgt.getBoundingClientRect();
          if (tr.width > 0 && tr.height > 0) {
            const px = cvLeft + (lx as number);
            const py = cvTop + (ly as number);
            const pad = lastAlert ? 18 : 0;
            hit =
              px >= tr.left - pad &&
              px <= tr.right + pad &&
              py >= tr.top - pad &&
              py <= tr.bottom + pad;
          }
        }
        alerting = hit;
        if (hit !== lastAlert) {
          lastAlert = hit;
          onAlertRef.current?.(hit);
        }
      }
    };

    const drawLens = (now: number) => {
      const cx = lx as number;
      const cy = ly as number;
      // The lens turns red both when magnifying a live compromise (inspecting)
      // and when it has caught the wired target (alerting).
      const alert = inspecting || alerting;
      ctx.save();
      ctx.beginPath();
      ctx.arc(cx, cy, LR, 0, 7);
      ctx.clip();
      ctx.globalAlpha = 1;
      ctx.fillStyle = "rgba(120,170,240,0.05)";
      ctx.fillRect(cx - LR, cy - LR, LR * 2, LR * 2);
      ctx.translate(cx, cy);
      ctx.scale(MAG, MAG);
      ctx.translate(-cx, -cy);
      drawWorld(now);
      ctx.restore();
      // reticle
      ctx.save();
      ctx.beginPath();
      ctx.arc(cx, cy, LR, 0, 7);
      ctx.clip();
      ctx.globalAlpha = 0.4;
      ctx.strokeStyle = "rgba(156,194,255,.55)";
      ctx.lineWidth = 1;
      ctx.beginPath();
      ctx.moveTo(cx - LR, cy);
      ctx.lineTo(cx + LR, cy);
      ctx.moveTo(cx, cy - LR);
      ctx.lineTo(cx, cy + LR);
      ctx.stroke();
      const sy = cy + Math.sin(now / 620) * LR * 0.82;
      ctx.globalAlpha = alert ? 0.5 : 0.28;
      ctx.strokeStyle = alert ? REDB : "#9cc2ff";
      ctx.beginPath();
      ctx.moveTo(cx - LR, sy);
      ctx.lineTo(cx + LR, sy);
      ctx.stroke();
      ctx.restore();
      // rim + glow
      ctx.save();
      ctx.shadowColor = alert ? RED : "#9cc2ff";
      ctx.shadowBlur = alert ? 20 : 14;
      ctx.strokeStyle = alert ? "rgba(255,138,144,.95)" : "rgba(156,194,255,.9)";
      ctx.lineWidth = 2.4;
      ctx.beginPath();
      ctx.arc(cx, cy, LR, 0, 7);
      ctx.stroke();
      ctx.restore();
      // handle
      const ang = 0.82;
      ctx.globalAlpha = 1;
      ctx.strokeStyle = "rgba(156,194,255,.85)";
      ctx.lineWidth = 4;
      ctx.lineCap = "round";
      ctx.beginPath();
      ctx.moveTo(cx + Math.cos(ang) * LR, cy + Math.sin(ang) * LR);
      ctx.lineTo(cx + Math.cos(ang) * (LR + LR * 0.72), cy + Math.sin(ang) * (LR + LR * 0.72));
      ctx.stroke();
      // label
      ctx.globalAlpha = 0.9;
      ctx.fillStyle = alert ? "#ffb9bd" : "#bcd6ff";
      ctx.font = "11px ui-monospace, monospace";
      ctx.textAlign = "center";
      ctx.fillText(
        alerting ? "eBPF sensor: anomaly" : inspecting ? "eBPF sensor: caught" : "eBPF sensor",
        cx,
        cy - LR - 9,
      );
      ctx.globalAlpha = 1;
    };

    let raf = 0;
    let running = false;
    const render = (now: number) => {
      if (rectDirty) {
        syncRect();
        rectDirty = false;
      }
      ctx.clearRect(0, 0, w, h);
      if (now >= nextSpawn) {
        spawn(now);
        nextSpawn = now + 3000 + Math.random() * 2400;
      }
      for (let i = inst.length - 1; i >= 0; i--) {
        const o = inst[i];
        const e = now - o.t0;
        if (e >= o.dur) {
          inst.splice(i, 1);
          continue;
        }
        o.a = envA(e, o.dur);
        o.pt = Math.max(0, Math.min(1, (e - 350) / (o.dur - 1550)));
      }
      drawWorld(now);
      updLens(now);
      drawLens(now);
    };
    const frame = (now: number) => {
      render(now);
      raf = requestAnimationFrame(frame);
    };

    const start = () => {
      // Gate only on reduced-motion and the offscreen observer - NOT document.hidden.
      // Real browsers already pause rAF for hidden tabs; visibilitychange (below)
      // stops us explicitly when the tab actually goes to the background. Gating the
      // initial start on document.hidden wrongly blocks environments that report the
      // page hidden while still running the frame loop.
      if (running || reduce || typeof requestAnimationFrame === "undefined") return;
      running = true;
      nextSpawn = performance.now();
      raf = requestAnimationFrame(frame);
    };
    const stop = () => {
      running = false;
      cancelAnimationFrame(raf);
    };

    const onMove = (e: PointerEvent) => {
      // Use the cached canvas origin instead of getBoundingClientRect here.
      // pointermove fires far more often than we paint, and the always-live log
      // feed on this page keeps layout perpetually dirty, so a rect read per move
      // forces a full synchronous reflow each time and starves the rAF/lens
      // pipeline - which is what made the follow feel laggy. The origin is
      // refreshed once per frame (rectDirty) and on scroll instead.
      const x = e.clientX - cvLeft;
      const y = e.clientY - cvTop;
      if (x < 0 || y < 0 || x > w || y > h) return;
      mx = x;
      my = y;
      manualUntil = performance.now() + 2600;
    };
    const onScroll = () => {
      rectDirty = true;
    };
    const onResize = () => {
      lx = ly = null;
      fit();
    };
    // This is a page-wide, always-on-screen fixed background, so it just runs
    // whenever the tab is visible. (IntersectionObserver is deliberately not used:
    // it doesn't reliably report position:fixed targets, which never move relative
    // to the viewport.) rAF self-throttles in hidden tabs; visibilitychange stops
    // it explicitly. The rAF guard in start() no-ops in environments without rAF.
    const onVisibility = () => {
      if (document.hidden) stop();
      else start();
    };

    fit();
    render(performance.now()); // paint one frame immediately, before the first rAF tick
    start();

    window.addEventListener("resize", onResize);
    window.addEventListener("pointermove", onMove, { passive: true });
    window.addEventListener("scroll", onScroll, { passive: true, capture: true });
    document.addEventListener("visibilitychange", onVisibility);

    if (reduce) {
      // one static frame: a couple of hints, lens parked on a compromise
      spawn(0);
      spawn(0);
      inst.forEach((o) => {
        o.a = 0.9;
        o.pt = 0.85;
      });
      const hh = inst.length ? hotOf(inst[0].scene) : null;
      if (hh) {
        lx = hh.x;
        ly = hh.y;
      }
      ctx.clearRect(0, 0, w, h);
      drawWorld(0);
      updLens(0);
      drawLens(0);
    }

    return () => {
      stop();
      window.removeEventListener("resize", onResize);
      window.removeEventListener("pointermove", onMove);
      window.removeEventListener("scroll", onScroll, true);
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, []);

  return (
    <canvas
      ref={ref}
      aria-hidden
      className={className ?? "pointer-events-none absolute inset-0 h-full w-full"}
    />
  );
}
