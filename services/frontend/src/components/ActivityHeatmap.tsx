import { useMemo } from "react";

import type { DayActivity } from "../lib/api";

// A GitHub-style contribution grid driven by REAL activity: each cell is one UTC
// day, shaded by how many lab sessions ended that day (server-provided in
// `activity`, the same signal as the streak). Unlike the old landing mock this
// invents nothing - an idle stretch renders empty, and the month labels and the
// session count are computed from the actual window, not hardcoded. Shared by
// the landing showcase and the public profile page so both read identically.

const LEVEL_BG = [
  "bg-[#0e1a2e]",
  "bg-[rgba(106,166,242,0.30)]",
  "bg-[rgba(106,166,242,0.52)]",
  "bg-[rgba(106,166,242,0.74)]",
  "bg-accent",
];
const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

// Work in UTC throughout so the client buckets days exactly as the server does
// (which aggregates ended_at at UTC midnight) - otherwise a session near
// midnight could land in a different cell on the two sides.
function utcMidnight(d: Date): Date {
  return new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate()));
}
function isoDay(d: Date): string {
  return d.toISOString().slice(0, 10);
}
function addDays(d: Date, n: number): Date {
  const out = new Date(d);
  out.setUTCDate(d.getUTCDate() + n);
  return out;
}

type Cell = { key: string; count: number; future: boolean };

export function ActivityHeatmap({
  activity,
  days = 140,
  className = "",
}: {
  activity: DayActivity[];
  days?: number;
  className?: string;
}) {
  const { columns, monthLabels, total, max } = useMemo(() => {
    const counts = new Map<string, number>();
    for (const a of activity) counts.set(a.date, a.count);

    const today = utcMidnight(new Date());
    // Columns are weeks starting Sunday; the last column holds the current
    // (possibly partial) week, with days past today left blank.
    const endSunday = addDays(today, -today.getUTCDay());
    const weeks = Math.ceil(days / 7) + 1;
    const start = addDays(endSunday, -(weeks - 1) * 7);
    const windowStart = addDays(today, -days);

    let total = 0;
    let max = 0;
    const columns: Cell[][] = [];
    for (let w = 0; w < weeks; w++) {
      const col: Cell[] = [];
      for (let dow = 0; dow < 7; dow++) {
        const d = addDays(start, w * 7 + dow);
        const future = d.getTime() > today.getTime();
        const count = future ? 0 : counts.get(isoDay(d)) ?? 0;
        if (!future && d.getTime() >= windowStart.getTime()) {
          total += count;
          if (count > max) max = count;
        }
        col.push({ key: isoDay(d), count, future });
      }
      columns.push(col);
    }

    // A month label sits above the first column whose Sunday falls in a new
    // month - the same placement GitHub uses.
    let lastMonth = -1;
    const monthLabels = columns.map((_, w) => {
      const sunday = addDays(start, w * 7);
      const m = sunday.getUTCMonth();
      if (m !== lastMonth) {
        lastMonth = m;
        return MONTHS[m];
      }
      return "";
    });

    return { columns, monthLabels, total, max: Math.max(1, max) };
  }, [activity, days]);

  // Shade relative to this operator's own busiest day so a light-but-real
  // schedule is still visible, rather than washing out against some global max.
  const level = (c: number) => (c <= 0 ? 0 : Math.min(4, Math.max(1, Math.ceil((c / max) * 4))));
  const months = Math.max(1, Math.round(days / 30));

  return (
    <div className={`overflow-x-auto text-center ${className}`}>
      <div className="inline-block text-left align-top">
        <div className="flex pb-2 pl-[28px]">
          <div className="grid flex-1 grid-flow-col auto-cols-[12px] gap-1 font-mono text-10 text-slate-500">
            {monthLabels.map((m, i) => (
              <div key={i} className="relative h-3">
                {m && <span className="absolute left-0 whitespace-nowrap">{m}</span>}
              </div>
            ))}
          </div>
        </div>
        <div className="flex gap-[6px]">
          <div className="grid w-[22px] grid-rows-[repeat(7,12px)] gap-1 text-right font-mono text-[9px] leading-3 text-slate-500">
            <span />
            <span>Mon</span>
            <span />
            <span>Wed</span>
            <span />
            <span>Fri</span>
            <span />
          </div>
          <div className="grid grid-flow-col grid-rows-[repeat(7,12px)] auto-cols-[12px] gap-1">
            {columns.map((col) =>
              col.map((cell) => (
                <div
                  key={cell.key}
                  title={cell.future ? undefined : `${cell.count} session${cell.count === 1 ? "" : "s"} · ${cell.key}`}
                  className={`h-3 w-3 rounded-[3px] ${cell.future ? "bg-transparent" : LEVEL_BG[level(cell.count)]}`}
                />
              )),
            )}
          </div>
        </div>
        <div className="mt-4 flex items-center justify-between gap-4 font-mono text-11 text-slate-500">
          <span>
            {total} lab session{total === 1 ? "" : "s"} in the last {months} month{months === 1 ? "" : "s"}
          </span>
          <span className="flex items-center gap-1">
            Less
            {[0, 1, 2, 3, 4].map((l) => (
              <i key={l} className={`inline-block h-2.5 w-2.5 rounded-[2px] ${LEVEL_BG[l]}`} />
            ))}
            More
          </span>
        </div>
      </div>
    </div>
  );
}
