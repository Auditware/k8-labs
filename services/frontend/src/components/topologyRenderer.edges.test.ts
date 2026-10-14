import { describe, expect, it } from "vitest";
import { renderTopology } from "./topologyRenderer";
import type { AdminTopology, TopoPod } from "../lib/api";

// Two relationships a lab map must express that an ordinary traffic edge cannot:
//
//   - a LOCAL privilege escalation, which begins and ends in the SAME pod. It
//     used to be declared as pod -> "ns:lab", which resolves to the hive centre,
//     so the arrowhead landed on whatever resource hex sat nearest that point (a
//     configmap) and read as "SUID -> configmaps". Now it is pod -> same pod and
//     draws as a loop.
//   - an eBPF sensor OBSERVING a pod, which is not traffic at all. It gets an
//     eye rather than an arrowhead, so it never reads as a network hop.
//
// jsdom reports every element as zero-sized, but hive/pod geometry is computed
// from constants (podCells), not measured, so the coordinates below are real.

const pod = (name: string, ns: string): TopoPod => ({
  name,
  app: name,
  workload: name,
  namespace: ns,
  phase: "Running",
  ready: "1/1",
  restarts: 0,
  containers: ["main"],
});

const payload: AdminTopology = {
  nodes: [{ name: "k8-labs", role: "labs", ready: true, kernel: "", runtime: "", os: "" }],
  namespaces: [{ name: "k8-labs", pods: [pod("k8-labs", "k8-labs")] }],
  edges: [],
  layers: [],
  dashboards: [],
  sessions: [
    {
      id: "your lab",
      module: "eBPF Detection 101",
      status: "running",
      cluster: {
        reachable: true,
        namespaces: [
          // Two configmaps on purpose: these are the hexes the old escape edge
          // mistakenly pointed at.
          { name: "lab", pods: [pod("workstation", "lab")], resources: { configMaps: 2 } },
          { name: "tetragon", pods: [pod("tetragon", "tetragon")] },
        ],
        edges: [
          { from: "workstation", to: "workstation", port: "SUID → euid 0 (detected)", kind: "escape" },
          { from: "tetragon", to: "workstation", kind: "observes" },
        ],
      },
    },
  ],
};

function withRender<T>(fn: (host: HTMLElement) => T): T {
  const host = document.createElement("div");
  document.body.appendChild(host);
  const cleanup = renderTopology(host, payload, { scoped: true });
  try {
    return fn(host);
  } finally {
    cleanup();
    host.remove();
  }
}

// "M{ax} {ay} Q {cx} {cy} {bx} {by}" -> the six numbers.
function pts(d: string) {
  const n = (d.match(/-?\d+(?:\.\d+)?/g) || []).map(Number);
  return { start: { x: n[0], y: n[1] }, ctrl: { x: n[2], y: n[3] }, end: { x: n[4], y: n[5] } };
}
const dist = (a: { x: number; y: number }, b: { x: number; y: number }) => Math.hypot(a.x - b.x, a.y - b.y);

describe("topology edge semantics", () => {
  it("draws a same-pod escalation as a loop, not a degenerate or stray arrow", () => {
    withRender((host) => {
      const path = [...host.querySelectorAll("path")].find((p) => p.getAttribute("stroke") === "#ff5b6b");
      expect(path, "no escape-coloured path was drawn").toBeTruthy();
      const { start, ctrl, end } = pts(path!.getAttribute("d") || "");

      // Not degenerate: the two feet are distinct points, so the curve exists.
      // (Routing a same-pod edge through the normal pod-to-pod arch collapsed to
      // a zero-length path and drew nothing at all.)
      expect(dist(start, end)).toBeGreaterThan(0);

      // It is a LOOP hugging one pod, not an arrow flying off to another hex:
      // the feet sit a pod's-width apart at most.
      expect(dist(start, end)).toBeLessThan(40);

      // ...and it bows far enough to be a visible arc rather than a flat nub.
      const apex = { x: 0.25 * start.x + 0.5 * ctrl.x + 0.25 * end.x, y: 0.25 * start.y + 0.5 * ctrl.y + 0.25 * end.y };
      const mid = { x: (start.x + end.x) / 2, y: (start.y + end.y) / 2 };
      expect(dist(apex, mid)).toBeGreaterThan(20);
    });
  });

  it("keeps the escalation's own label on the loop", () => {
    withRender((host) => {
      expect(host.innerHTML).toContain("SUID → euid 0 (detected)");
    });
  });

  it("draws the sensor's observation with an eye, not an arrowhead", () => {
    withRender((host) => {
      const path = [...host.querySelectorAll("path")].find((p) => p.getAttribute("marker-end") === "url(#tp-eye)");
      expect(path, "no observes edge was drawn").toBeTruthy();
      expect(path!.getAttribute("stroke")).toBe("var(--tp-accent)");
      // Passive gaze, not flowing traffic: finer dash and no march animation.
      expect(path!.getAttribute("stroke-dasharray")).toBe("2 4");
      expect(path!.querySelector("animate")).toBeNull();
    });
  });

  it("still routes a cross-pod edge normally (the loop case must not over-trigger)", () => {
    withRender((host) => {
      const path = [...host.querySelectorAll("path")].find((p) => p.getAttribute("marker-end") === "url(#tp-eye)");
      const { start, end } = pts(path!.getAttribute("d") || "");
      // tetragon -> workstation crosses namespaces, so its ends are far apart.
      expect(dist(start, end)).toBeGreaterThan(60);
    });
  });

  it("lists the observes key in the legend only because an eBPF edge is drawn", () => {
    withRender((host) => {
      expect(host.innerHTML).toContain("observes (eBPF)");
    });
  });
});
