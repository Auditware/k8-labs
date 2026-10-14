import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { getAdminTopology, type AdminTopology } from "../lib/api";
import { palette } from "../lib/tokens";

// The capacity + health summary above the topology map. Grouped horizontal bar
// lists rather than a wall of donuts: fewer, denser charts that still carry
// every data point.
//
// The grouping is by SHARED MEANING, which is what lets several metrics sit on
// one chart honestly. "Reserved" bars are consumption (fuller = closer to the
// wall); "Fleet" bars are healthy-fraction (fuller = better). Those two
// polarities must not share an axis - a full bar would mean opposite things -
// so they are two charts, but within each the bars are the same 0-100% reading
// and directly comparable. Sessions is a bare count with no ceiling, so it is a
// figure, not a bar (the skill's "not everything is a chart").
//
// Shares TopologyMap's query key, so react-query dedupes the poll. All values
// are scheduler-visible (reservations, allocatable, ready-counts); there is no
// live host telemetry (the kubelet stats API 500s under Kata).
export default function CapacitySummary() {
  const { data } = useQuery({
    queryKey: ["admin-topology"],
    queryFn: getAdminTopology,
    refetchInterval: 5_000,
    refetchIntervalInBackground: true,
    staleTime: 5_000,
    // Keep the last snapshot on refetch/return so the section never blanks back
    // to a skeleton once it has loaded - only the very first cold load skeletons.
    placeholderData: (prev) => prev,
  });
  // Render the frame immediately rather than lazily popping in under the
  // "Capacity & health" heading a second later: the columns/bars are known, so
  // show them as a skeleton and let the numbers fill in.
  if (!data) return <SummarySkeleton />;
  return <Summary data={data} />;
}

// The chart frame with its real labels but muted, value-less tracks. Same
// dimensions as Summary (column width/height, bar rows) so the numbers filling
// in causes no layout shift.
function SummarySkeleton() {
  const COL = "2.75rem";
  return (
    <div className="flex flex-col gap-6 sm:flex-row sm:items-start sm:justify-center sm:gap-16">
      <div>
        <div className="mb-2 text-10 uppercase tracking-wide text-slate-500">Reserved · labs node</div>
        <div className="w-fit">
          <div className="flex items-end gap-6" style={{ height: 88 }}>
            {["Memory", "CPU", "Sessions"].map((l) => (
              <div
                key={l}
                className="animate-pulse rounded-md"
                style={{ width: COL, height: "100%", background: TONE.track }}
              />
            ))}
          </div>
          <div className="mt-2 flex gap-6">
            {["Memory", "CPU", "Sessions"].map((l) => (
              <div key={l} className="flex flex-col items-center leading-tight" style={{ width: COL }}>
                <span className="text-11 text-slate-400">{l}</span>
                <span className="font-mono text-10 text-slate-600">-</span>
              </div>
            ))}
          </div>
        </div>
      </div>
      <div className="w-full sm:w-[22rem]">
        <div className="mb-2 text-10 uppercase tracking-wide text-slate-500">Fleet health</div>
        <div className="flex flex-col gap-2">
          {["Nodes", "Pods", "ArgoCD", "Cert"].map((l) => (
            <div key={l} className="grid grid-cols-[3.5rem_1fr_auto] items-center gap-2.5">
              <span className="text-11 text-slate-400">{l}</span>
              <div className="h-1.5 animate-pulse rounded-full" style={{ background: TONE.track }} />
              <span className="min-w-[5.5rem] text-right font-mono text-11 text-slate-600">-</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

// Runtime fills (chart bars are sized/coloured in inline style, so these are
// hex, not classes) drawn from the shared palette - same source as the
// bg-status-* / border-hairline utilities.
const TONE = {
  ok: palette.ok,
  warn: palette.warn,
  serious: palette.danger,
  track: palette.hairline,
} as const;
type Tone = keyof typeof TONE;

const GIB = 1024 ** 3;
// A normal lab's memory reservation, mirroring backend labs.SessionMemory (the
// non-eBPF value). Used only to turn the raw session COUNT into a "slots used /
// slots that fit" bar - a display estimate, not enforcement, so a small drift
// from the backend constant is harmless. eBPF labs cost more, so the fit count
// is an optimistic upper bound; the value carries a ~ to say so.
const NORMAL_SESSION_GIB = 1.5;
const fmtGiB = (b: number) => {
  const g = b / GIB;
  return g >= 10 ? Math.round(g).toString() : g.toFixed(1);
};
const fmtCores = (m: number) => {
  const c = m / 1000;
  return c >= 10 ? Math.round(c).toString() : c.toFixed(c % 1 === 0 ? 0 : 1);
};
// Consumption: empty is calm, full is alarming.
const utilTone = (pct: number): Tone => (pct >= 90 ? "serious" : pct >= 70 ? "warn" : "ok");
// Healthy fraction: the reverse - full is calm, empty is alarming.
const healthTone = (pct: number): Tone => (pct >= 90 ? "ok" : pct >= 50 ? "warn" : "serious");

// A per-user slice of a reserved column: pct is its share of the SAME ceiling
// the whole column measures against, so the segments sum to the column's pct.
// valueText is the hover reveal (this user's amount, in the column's unit).
type Seg = { user: string; pct: number; valueText: string };
// system is the muted non-lab base segment (platform pods); it stacks below the
// lab usage (single fill or per-user segments) against the same ceiling.
type Bar = {
  label: string;
  value: string;
  pct: number;
  tone: Tone;
  segments?: Seg[];
  system?: { pct: number; text: string };
};

function Summary({ data }: { data: AdminTopology }) {
  const cap = data.capacity;
  const nodes = data.nodes ?? [];
  const sessions = data.sessions ?? [];
  const pods = (data.namespaces ?? []).flatMap((ns) => ns.pods ?? []);
  const sig = data.signals;

  // Reserved: memory toward min(budget, node) - whichever binds first - and CPU
  // toward node allocatable. See the topology.Capacity comment for why the
  // smaller of budget/node is the honest memory ceiling.
  const reserved: Bar[] = [];
  if (cap) {
    const memNode = cap.memAllocatableBytes;
    const memCeil =
      cap.memBudgetBytes > 0 && memNode > 0
        ? Math.min(cap.memBudgetBytes, memNode)
        : cap.memBudgetBytes || memNode;
    const memPct = memCeil > 0 ? (cap.memReservedBytes / memCeil) * 100 : 0;
    const cpuPct = cap.cpuAllocatableMillis > 0 ? (cap.cpuReservedMillis / cap.cpuAllocatableMillis) * 100 : 0;
    // Non-lab platform baseline (tetragon, CNI, coredns, ...) against the same
    // ceiling, so it stacks under the lab usage. The column TONE keys off the
    // combined fill, not labs alone: a node three-quarters full of platform
    // pods has little headroom for a lab even at 0 lab reservation, and the
    // colour should say so.
    const memSysPct = memCeil > 0 ? (cap.memSystemReservedBytes / memCeil) * 100 : 0;
    const cpuSysPct = cap.cpuAllocatableMillis > 0 ? (cap.cpuSystemReservedMillis / cap.cpuAllocatableMillis) * 100 : 0;
    // How many lab slots the binding memory ceiling holds - the count that turns
    // "1 session" into "1 of the ~N that fit", i.e. how much lab headroom is
    // left. Same consumption polarity as the memory/cpu bars (fuller = tighter),
    // so it belongs on this chart.
    const maxSessions = memCeil > 0 ? Math.floor(memCeil / (NORMAL_SESSION_GIB * GIB)) : 0;
    const sessPct = maxSessions > 0 ? (sessions.length / maxSessions) * 100 : 0;
    // Per-user segments: each user's share of the SAME ceiling the column uses,
    // so the stack sums to the column total. The user string "" is the folded
    // "others" row (see topology.CapacityUser); shown as "others". Undefined
    // when the backend sent no breakdown - the column then renders as one fill.
    const byUser = cap.byUser ?? [];
    const seg = (
      ceil: number,
      amount: (u: (typeof byUser)[number]) => number,
      fmt: (n: number) => string,
    ): Seg[] | undefined =>
      byUser.length && ceil > 0
        ? byUser.map((u) => ({
            user: u.user || "others",
            pct: (amount(u) / ceil) * 100,
            valueText: fmt(amount(u)),
          }))
        : undefined;
    // Compact values: these sit under narrow columns, so the metric label
    // carries the unit (GiB under Memory, cores under CPU, slots under Sessions)
    // and the number stays short.
    reserved.push(
      {
        label: "Memory",
        value: `${fmtGiB(cap.memReservedBytes)}/${fmtGiB(memCeil)}G`,
        pct: memPct,
        tone: utilTone(memPct + memSysPct),
        segments: seg(memCeil, (u) => u.memBytes, (n) => `${fmtGiB(n)} GiB`),
        system: memSysPct > 0 ? { pct: memSysPct, text: `${fmtGiB(cap.memSystemReservedBytes)} GiB` } : undefined,
      },
      {
        label: "CPU",
        value: `${fmtCores(cap.cpuReservedMillis)}/${fmtCores(cap.cpuAllocatableMillis)}`,
        pct: cpuPct,
        tone: utilTone(cpuPct + cpuSysPct),
        segments: seg(cap.cpuAllocatableMillis, (u) => u.cpuMillis, (n) => `${fmtCores(n)} cores`),
        system: cpuSysPct > 0 ? { pct: cpuSysPct, text: `${fmtCores(cap.cpuSystemReservedMillis)} cores` } : undefined,
      },
      {
        label: "Sessions",
        value: `${sessions.length}/~${maxSessions}`,
        pct: sessPct,
        tone: utilTone(sessPct),
        segments: seg(maxSessions, (u) => u.sessions, (n) => `${n} lab${n === 1 ? "" : "s"}`),
      },
    );
  }

  // Fleet: every bar a healthy-fraction. Nodes/pods/apps as ready-of-total,
  // cert as runway toward a 90-day horizon. ArgoCD keeps its own tone (drift is
  // a warning, not a critical) rather than the generic health scale, so an
  // all-drifted 0% doesn't read as an outage.
  const nodesReady = nodes.filter((n) => n.ready).length;
  const podsRunning = pods.filter((p) => p.phase === "Running").length;
  const argoSynced = sig?.argo?.synced ?? 0;
  const argoTotal = sig?.argo?.total ?? 0;
  const argoDrift = sig?.argo?.outOfSync?.length ?? Math.max(0, argoTotal - argoSynced);
  const certDays = sig?.certs?.soonestDays ?? -1;

  const fleet: Bar[] = [
    {
      label: "Nodes",
      value: `${nodesReady}/${nodes.length} ready`,
      pct: nodes.length ? (nodesReady / nodes.length) * 100 : 0,
      tone: healthTone(nodes.length ? (nodesReady / nodes.length) * 100 : 0),
    },
    {
      label: "Pods",
      value: `${podsRunning}/${pods.length} running`,
      pct: pods.length ? (podsRunning / pods.length) * 100 : 0,
      tone: healthTone(pods.length ? (podsRunning / pods.length) * 100 : 0),
    },
  ];
  if (sig?.argo) {
    // Framed as synced-of-total like the other fleet bars, so the bar (a healthy
    // fraction) and its label agree: an empty bar reads as "0 synced", not as
    // missing data. The drift count rides along in the label when non-zero.
    fleet.push({
      label: "ArgoCD",
      value: argoDrift > 0 ? `${argoSynced}/${argoTotal} · ${argoDrift} drift` : `${argoSynced}/${argoTotal} synced`,
      pct: argoTotal ? (argoSynced / argoTotal) * 100 : 100,
      tone: argoDrift > 0 ? "warn" : "ok",
    });
  }
  if (certDays >= 0) {
    fleet.push({
      label: "Cert",
      value: `${certDays}d left`,
      pct: Math.min(100, (certDays / 90) * 100),
      tone: certDays <= 3 ? "serious" : certDays <= 14 ? "warn" : "ok",
    });
  }

  return (
    // Two deliberately different chart forms. Reserved is consumption toward a
    // shared ceiling, so it reads as columns filling up under a danger line;
    // fleet health is a set of independent ratios, so it reads as horizontal
    // bars. The forms themselves signal "these are two different kinds of thing."
    //
    // A flex row rather than a 50/50 grid: the reserved columns are narrow, so a
    // half-width cell would leave a chasm before fleet health starts. The two
    // groups keep a fixed gap and centre as one cluster, so the slack splits
    // evenly on both sides rather than pooling to the right.
    <div className="flex flex-col gap-6 sm:flex-row sm:items-start sm:justify-center sm:gap-16">
      {reserved.length > 0 && <ColumnChart title="Reserved · labs node" bars={reserved} thresholdPct={90} />}
      <div className="w-full sm:w-[22rem]">
        <BarGroup title="Fleet health" bars={fleet} />
      </div>
    </div>
  );
}

// The reserved-capacity chart: vertical columns filling toward a shared ceiling,
// with a dashed danger line at the admission threshold. Distinct from the fleet
// bars on purpose - a column climbing toward a line reads as "filling up," which
// is what consumption is; the line is where the next lab starts getting refused.
//
// Each column is one status colour (utilTone of its total), and when a per-user
// breakdown exists it is subdivided into SHADES of that same colour - so status
// stays the truth of the column while user attribution rides underneath, and
// identity is revealed on hover rather than spent on a second colour dimension.
function ColumnChart({
  title,
  bars,
  thresholdPct,
}: {
  title: string;
  bars: Bar[];
  thresholdPct: number;
}) {
  const H = 88;
  const COL = "2.75rem";
  const [tip, setTip] = useState<{ x: number; y: number; text: string } | null>(null);

  return (
    <div>
      {/* Hover affordance: dim the column's other segments and ring the hovered
          one, so the user's slice pops without a colour change. Scoped by class;
          one ColumnChart on the page. */}
      <style>{`
        .cap-col:hover .cap-seg:not(:hover){filter:brightness(.5)}
        .cap-seg{outline:1.5px solid transparent;outline-offset:-1px;transition:filter .12s,outline-color .12s}
        .cap-seg:hover{outline-color:rgba(255,255,255,.85)}
      `}</style>
      <div className="mb-2 text-10 uppercase tracking-wide text-slate-500">{title}</div>
      {/* fit-content so the danger line spans exactly the columns, not the empty
          space beside them - the columns cluster rather than spreading across the
          panel width. */}
      <div className="w-fit">
        <div className="relative flex items-end gap-6" style={{ height: H }}>
          {/* Danger line: where reserved usage is close enough to the ceiling
              that the next session may be refused. One line across all columns
              since each is a % of its own ceiling. */}
          <div
            className="pointer-events-none absolute inset-x-0 border-t border-dashed border-status-warn/45"
            style={{ bottom: `${thresholdPct}%` }}
          />
          {bars.map((b) => (
            <div
              key={b.label}
              className="cap-col relative overflow-hidden rounded-md"
              style={{ width: COL, height: "100%", background: TONE.track }}
            >
              {/* One bottom-up stack: the muted system baseline sits at the
                  floor, the lab usage (per-user shades, heaviest/darkest first,
                  or a single fill when there's no breakdown) climbs above it.
                  Every height is a share of the same ceiling, so system + labs
                  leave exactly the true free headroom at the top. */}
              <div className="absolute inset-0 flex flex-col-reverse">
                {b.system && b.system.pct > 0 && (
                  <div
                    className="cap-seg"
                    style={{ height: `${Math.max(0, Math.min(100, b.system.pct))}%`, background: palette.system }}
                    onMouseMove={(e) => setTip({ x: e.clientX, y: e.clientY, text: `system · ${b.system!.text}` })}
                    onMouseLeave={() => setTip(null)}
                  />
                )}
                {b.segments && b.segments.length > 0
                  ? b.segments.map((s, i) => (
                      <div
                        key={s.user + i}
                        className="cap-seg"
                        style={{ height: `${Math.max(0, Math.min(100, s.pct))}%`, background: shade(TONE[b.tone], i) }}
                        onMouseMove={(e) => setTip({ x: e.clientX, y: e.clientY, text: `${s.user} · ${s.valueText}` })}
                        onMouseLeave={() => setTip(null)}
                      />
                    ))
                  : b.pct > 0 && (
                      // No breakdown (idle, or an older backend): a single fill,
                      // floored so a small-but-present value stays a visible sliver.
                      <div
                        className="cap-seg transition-[height] duration-500"
                        style={{ height: `${Math.max(4, Math.min(100, b.pct))}%`, background: TONE[b.tone] }}
                      />
                    )}
              </div>
            </div>
          ))}
        </div>
        <div className="mt-2 flex gap-6">
          {bars.map((b) => (
            <div key={b.label} className="flex flex-col items-center leading-tight" style={{ width: COL }}>
              <span className="text-11 text-slate-400">{b.label}</span>
              <span className="whitespace-nowrap font-mono text-10 text-slate-500">{b.value}</span>
            </div>
          ))}
        </div>
      </div>
      {tip && (
        <div
          className="pointer-events-none fixed z-50 -translate-x-1/2 -translate-y-full rounded-md border border-slate-600 bg-slate-900 px-2 py-1 text-11 text-slate-100 shadow-lg"
          style={{ left: tip.x, top: tip.y - 8 }}
        >
          {tip.text}
        </div>
      )}
    </div>
  );
}

// Lighten a status-tone hex toward white for the i-th stacked user segment:
// i=0 (heaviest user, at the bottom) stays near the base tone, each higher slice
// gets lighter, capped so even a long stack keeps segments within the same hue
// and readable on the dark surface.
function shade(hex: string, i: number): string {
  const n = parseInt(hex.slice(1), 16);
  const r = n >> 16;
  const g = (n >> 8) & 255;
  const b = n & 255;
  const t = Math.min(0.06 + i * 0.24, 0.7);
  const mix = (c: number) => Math.round(c + (255 - c) * t);
  return `rgb(${mix(r)}, ${mix(g)}, ${mix(b)})`;
}

// A titled list of thin horizontal bars sharing one 0-100% track width, so the
// bars line up and read as one chart. Label left, bar centre, value right.
function BarGroup({ title, bars }: { title: string; bars: Bar[] }) {
  return (
    <div>
      <div className="mb-2 text-10 uppercase tracking-wide text-slate-500">{title}</div>
      <div className="flex flex-col gap-2">
        {bars.map((b) => (
          <div key={b.label} className="grid grid-cols-[3.5rem_1fr_auto] items-center gap-2.5">
            <span className="text-11 text-slate-400">{b.label}</span>
            <div className="h-1.5 overflow-hidden rounded-full" style={{ background: TONE.track }}>
              <div
                className="h-full rounded-full transition-[width] duration-500"
                // Floor a non-zero bar at 3% so a small-but-present value is
                // still a visible sliver, never an empty track that reads as 0.
                style={{
                  width: `${b.pct > 0 ? Math.max(3, Math.min(100, b.pct)) : 0}%`,
                  background: TONE[b.tone],
                }}
              />
            </div>
            <span className="min-w-[5.5rem] text-right font-mono text-11 text-slate-300">{b.value}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
