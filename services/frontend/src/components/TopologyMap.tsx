import { useEffect, useRef } from "react";
import { useQuery } from "@tanstack/react-query";
import { getAdminTopology } from "../lib/api";
import { renderTopology } from "./topologyRenderer";

// First-load indicator. Deliberately just a spinner: the build is ~1s now that
// the client-go rate limiting and the sequential namespace walk are fixed (see
// internal/topology/builder.go), so there's no long wait left to dress up. A
// layout-shaped skeleton would be pretending the load is slower than it is.
function TopologySpinner() {
  return (
    <div className="absolute inset-0 flex flex-col items-center justify-center gap-3">
      <span
        className="h-7 w-7 animate-spin rounded-full border-2 border-slate-700 border-t-blue-400"
        role="status"
        aria-label="Loading topology"
      />
      <p className="text-xs text-slate-600">Loading topology…</p>
    </div>
  );
}

// Live topology map. react-query owns the fetch/poll; the imperative renderer
// (topologyRenderer.ts) owns the DOM. refetchIntervalInBackground keeps it
// updating even when the tab is unfocused - without it react-query pauses the
// interval on blur, which is the stall we hit on the provisioning spinner.
export default function TopologyMap() {
  const ref = useRef<HTMLDivElement>(null);
  const { data, isLoading, isFetching, isError, error } = useQuery({
    queryKey: ["admin-topology"],
    queryFn: getAdminTopology,
    // 5s, not 15s: the build is ~1s (live K8s read, no cache) so a session that
    // ends is reflected within one short tick instead of feeling laggy. Lab
    // lifecycle actions also invalidate this key (see LabPage's start/stop) so
    // the admin who just closed a lab sees it drop off immediately on navigate,
    // rather than waiting out a poll. Kept above ~3s because each poll fans out
    // a multi-namespace pod/node list.
    refetchInterval: 5_000,
    refetchIntervalInBackground: true,
    // Match the interval: a map older than one tick is stale, so navigating to
    // the dashboard refetches on mount rather than painting a >5s-old cache.
    staleTime: 5_000,
  });

  useEffect(() => {
    if (!ref.current || !data) return;
    return renderTopology(ref.current, data);
  }, [data]);

  // The node/pod/session/ArgoCD/cert rollup that used to sit here as a chip row
  // now lives in CapacitySummary (rendered above this map on the admin page),
  // where it is charted alongside CPU and memory reservation rather than shown
  // as bare numbers. Keeping it in one place avoids the same facts in two forms.

  return (
    <div>
      <div className="relative h-[82vh] min-h-[600px] overflow-hidden rounded-xl border border-hairline-strong">
        <div ref={ref} className="h-full" />
        {/* First load only. Refetches never reach here - `data` persists, so
            the live map stays up and only the pill below changes. */}
        {isLoading && <TopologySpinner />}

        {/* Background refresh indicator. Deliberately a small corner pill
            rather than an overlay: the poll must never obscure the map the
            user is reading. */}
        {!isLoading && isFetching && (
          <div className="absolute right-3 top-3 flex items-center gap-1.5 rounded-full border border-slate-700 bg-slate-900/90 px-2.5 py-1 text-11 text-slate-400">
            <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-blue-400" />
            Refreshing
          </div>
        )}
        {isError && (
          <div className="absolute inset-0 flex items-center justify-center text-sm text-red-400">
            Failed to load topology{error instanceof Error ? `: ${error.message}` : ""}
          </div>
        )}
      </div>
    </div>
  );
}
