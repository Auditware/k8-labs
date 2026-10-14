import { useEffect, useRef } from "react";
import { useQuery } from "@tanstack/react-query";
import { getSessionTopology, type AdminTopology, type ScopedTopology } from "../lib/api";
import { renderTopology } from "./topologyRenderer";

// The student's view of their own lab cluster.
//
// It reuses the admin renderer unchanged - same hex/hive visual language, same
// edge routing, same popovers - so the two maps look like one product. The
// sharing is deliberately at the RENDERER, not at the data: the payload comes
// from GET /sessions/{id}/topology, which is built from the student's nested
// kubeconfig alone and structurally cannot contain outer-cluster data (see
// internal/topology/scoped.go). Sharing a renderer is safe; sharing a data
// source would not be.
//
// The adapter below is the only new logic, and it exists because the renderer
// draws nested clusters as a region hanging off a node - so a student's map
// needs exactly one node to hang from. That node is the opaque boundary: a
// single featureless "k8-labs" pod standing for everything outside their lab.
// It carries no name, version, address or workload of ours.

// toAdminShape adapts the scoped payload into what renderTopology expects.
//
// Everything the admin payload would carry and a student must not see - real
// node names, kernel/runtime/OS, host substrate layers, dashboard links,
// ArgoCD/cert signals - is simply absent here, because the scoped payload has
// nowhere to put it. This function invents nothing beyond the boundary marker.
function toAdminShape(scoped: ScopedTopology, moduleName: string): AdminTopology {
  const boundaryPod = {
    name: "k8-labs",
    app: "k8-labs",
    workload: "k8-labs",
    namespace: "k8-labs",
    phase: "Running",
    ready: "1/1",
    restarts: 0,
    containers: ["platform"],
  };

  return {
    // One synthetic node. `ready: true` and an empty kernel string: the
    // renderer shows kernel in the node header, and a student has no business
    // knowing which kernel the platform runs - that is the containment
    // boundary the labs are built on.
    nodes: [{ name: "k8-labs", role: "labs", ready: true, kernel: "", runtime: "", os: "" }],
    // The boundary hive. This is the "one fake pod for clarity" - it exists so
    // the student's cluster is visibly attached to something, rather than
    // floating with no context.
    namespaces: [{ name: "k8-labs", pods: [boundaryPod] }],
    edges: scoped.edges ?? [],
    // Empty by construction, not by filtering. The scoped payload has no
    // layers, dashboards or signals to pass through.
    layers: [],
    dashboards: [],
    sessions: [
      {
        id: "your lab",
        module: moduleName,
        status: "running",
        cluster: {
          reachable: !scoped.error,
          namespaces: scoped.namespaces ?? [],
          // Edges that live INSIDE the student's cluster: the attack path
          // ("escape") and, for eBPF modules, the sensor's "observes" link. Both
          // resolve against nested pods, so they belong here rather than in the
          // top-level edges above (which resolve against the outer boundary hive
          // only and would silently drop them).
          edges: scoped.edges?.filter((e) => e.kind === "escape" || e.kind === "observes") ?? [],
          error: scoped.error,
        },
      },
    ],
  };
}

export default function ScopedTopologyMap({
  sessionId,
  moduleName,
}: {
  sessionId: string;
  moduleName: string;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["session-topology", sessionId],
    queryFn: () => getSessionTopology(sessionId),
    // Adaptive interval so the map paints quickly. Until the nested cluster is
    // reachable (the session is still provisioning, or the kubeconfig hasn't
    // been fetched yet) the endpoint returns an `error` placeholder - poll that
    // every 2s so the real map appears within a couple of seconds of the lab
    // coming up, not up to a full slow-interval later. Once it's reachable,
    // back off to 15s: a student's cluster only changes when they change it,
    // and each poll dials their nested API, so there's no point hammering it.
    refetchInterval: (query) => (query.state.data?.error ? 2_000 : 15_000),
    // Keep polling while provisioning even if the tab isn't focused, so
    // switching to Topology right after start doesn't find a stale placeholder.
    refetchIntervalInBackground: true,
    staleTime: 2_000,
  });

  useEffect(() => {
    if (!ref.current || !data) return;
    // scoped: true suppresses the renderer's own static chrome - the access
    // lane naming an admin with an allowlisted IP, the host-substrate band,
    // and the isolation-wall panel naming the runtime and NetworkPolicy
    // architecture. None of that comes from the payload, so the scoped
    // builder alone does not remove it.
    return renderTopology(ref.current, toAdminShape(data, moduleName), { scoped: true });
  }, [data, moduleName]);

  return (
    <div className="relative h-full w-full">
      <div ref={ref} className="h-full w-full" />
      {isLoading && (
        <div className="absolute inset-0 flex flex-col items-center justify-center gap-3">
          <span
            className="h-7 w-7 animate-spin rounded-full border-2 border-slate-700 border-t-blue-400"
            role="status"
            aria-label="Loading your lab topology"
          />
          <p className="text-xs text-slate-600">Mapping your lab…</p>
        </div>
      )}
      {/* A not-yet-reachable cluster is normal (still provisioning, or just torn
          down), so it reads as a status line rather than a failure. */}
      {!isLoading && data?.error && (
        <div className="absolute inset-x-0 top-3 mx-auto w-fit rounded-full border border-slate-700 bg-slate-900/90 px-3 py-1 text-11 text-slate-400">
          {data.error}
        </div>
      )}
      {isError && (
        <div className="absolute inset-0 flex items-center justify-center px-6 text-center text-sm text-red-400">
          Couldn't load your lab topology
          {error instanceof Error ? `: ${error.message}` : ""}
        </div>
      )}
    </div>
  );
}
