import { describe, expect, it } from "vitest";
import { renderTopology } from "./topologyRenderer";
import type { AdminTopology } from "../lib/api";

// The renderer draws chrome that NO payload controls: an access lane naming an
// admin reaching the platform from an allowlisted IP, a host-substrate band, and
// an isolation-wall panel naming the container runtime and our NetworkPolicy
// architecture.
//
// The backend's scoped payload is clean - internal/topology/scoped.go makes that
// structural, and its own tests prove it - and the student's map was STILL
// disclosing infrastructure, because this half of the picture is drawn
// client-side from constants. Found by opening the real running app, not by any
// payload test. These assertions stop it coming back.

const payload: AdminTopology = {
  nodes: [{ name: "k8-labs", role: "labs", ready: true, kernel: "", runtime: "", os: "" }],
  namespaces: [
    {
      name: "k8-labs",
      pods: [
        {
          name: "k8-labs",
          app: "k8-labs",
          workload: "k8-labs",
          namespace: "k8-labs",
          phase: "Running",
          ready: "1/1",
          restarts: 0,
          containers: ["platform"],
        },
      ],
    },
  ],
  edges: [],
  // Layers are passed on purpose even though the real adapter sends none:
  // scoped mode must refuse to draw them regardless of what it is handed, so
  // this keeps holding if the adapter changes later.
  layers: [{ nodeRole: "labs", title: "Firecracker microVM", sub: "kata-fc", kind: "wall" }],
  dashboards: [],
  sessions: [
    {
      id: "your lab",
      module: "Namespace Escape 101",
      status: "running",
      cluster: {
        reachable: true,
        namespaces: [
          {
            name: "dev",
            pods: [
              {
                name: "foothold",
                app: "foothold",
                workload: "foothold",
                namespace: "dev",
                phase: "Running",
                ready: "1/1",
                restarts: 0,
                containers: ["shell"],
              },
            ],
          },
        ],
        edges: [],
      },
    },
  ],
};

function renderInto(scoped: boolean): string {
  const host = document.createElement("div");
  // jsdom reports every element as zero-sized, which affects layout maths but
  // not which text the renderer emits - and text is all this checks.
  document.body.appendChild(host);
  const cleanup = renderTopology(host, payload, scoped ? { scoped: true } : {});
  const html = host.innerHTML;
  cleanup();
  host.remove();
  return html;
}

describe("renderTopology scoped mode", () => {
  const forbidden = [
    "host substrate", // the machine under the platform
    "isolation walls", // names the containment the student is inside
    "runc - no microVM", // ...and whether it is the weak local one
    "Firecracker", // ...or the real one
    "session-runner NetworkPolicy", // our network architecture
    "blocks outer API + backend",
    "allowlisted IP", // our administrative access path
    "public / host access",
  ];

  it("draws none of the platform's own chrome", () => {
    const html = renderInto(true);
    for (const s of forbidden) {
      expect(html, `scoped map disclosed ${JSON.stringify(s)}`).not.toContain(s);
    }
  });

  it("still shows the student their own cluster", () => {
    // Suppressing chrome must not suppress the lab itself, or the tab is empty
    // and the first test would pass for the wrong reason.
    const html = renderInto(true);
    expect(html).toContain("dev");
    expect(html).toContain("foothold");
  });

  it("keeps the chrome for admins", () => {
    // Otherwise the first test could pass simply because the renderer stopped
    // drawing this for everyone.
    const html = renderInto(false);
    expect(html).toContain("host substrate");
    expect(html).toContain("isolation walls");
  });
});
