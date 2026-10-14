import { useId, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Upload, Check, AlertTriangle, Clock, X } from "lucide-react";
import { listDrivers, uploadDriver, type Driver } from "../lib/api";
import { Banner } from "./ui/Banner";

// DriverPicker is the research lab's pre-start driver selector: it lists the
// vetted catalog the user may load, lets them check which to mount, and accepts
// a compiled .ko (click or drag-drop) for admin vetting. Selection is only
// actionable before the session starts (drivers are mounted at provision), so
// `disabled` locks it to a read-only summary once a session is running.
//
// State is lifted: the parent (LabPage) owns `selected` so it can pass the ids
// into createSession. See docs/architecture/research-driver-catalog.md.
//
// The table is deliberately dense rather than card-shaped: the shared catalog
// grows over time and a researcher scanning it cares about a few comparable
// facts per row (does it load here, what licence, how big), which columns show
// far better than stacked prose.

export default function DriverPicker({
  selected,
  onChange,
  disabled,
  expectedVermagic,
}: {
  selected: string[];
  onChange: (ids: string[]) => void;
  disabled?: boolean;
  // The vermagic this lab's guest kernel demands (Module.expectedVermagic).
  // Optional: when the backend doesn't supply one the compatibility column is
  // hidden entirely rather than guessed, since a wrong verdict here is worse
  // than none - it would either hide a loadable driver or bless an unloadable one.
  expectedVermagic?: string;
}) {
  const queryClient = useQueryClient();
  const fileInput = useRef<HTMLInputElement>(null);
  const [dragging, setDragging] = useState(false);
  const baseId = useId();

  const { data: drivers, isPending, isError } = useQuery({
    queryKey: ["drivers"],
    queryFn: listDrivers,
  });

  const upload = useMutation({
    mutationFn: (file: File) => uploadDriver(file),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["drivers"] }),
  });

  const toggle = (id: string) => {
    if (disabled) return;
    onChange(selected.includes(id) ? selected.filter((x) => x !== id) : [...selected, id]);
  };

  // Vermagic is the ONLY load-time compatibility check on the research kernels
  // (they build with MODVERSIONS off), so a mismatch here is a certain insmod
  // failure, not a hint. Compared trimmed because `modinfo -F vermagic` prints a
  // trailing space that the stored value keeps.
  const fits = (d: Driver) =>
    !!expectedVermagic && d.vermagic.trim() === expectedVermagic.trim();

  // Approved drivers are selectable; the user's own pending/denied uploads are
  // listed under them so vetting status (and why something was denied) is
  // visible without cluttering the selectable rows.
  const approved = (drivers ?? []).filter((d) => d.status === "approved");
  const mine = (drivers ?? []).filter((d) => d.status !== "approved");

  const send = (file?: File | null) => {
    if (file && !disabled) upload.mutate(file);
  };

  return (
    <div className="mt-5 overflow-hidden rounded-lg border border-hairline bg-surface/60">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1 border-b border-hairline px-3 py-2">
        <span className="font-mono text-11 uppercase tracking-wider text-accent">Drivers</span>
        {expectedVermagic && (
          <span className="truncate font-mono text-10 text-slate-600">{expectedVermagic}</span>
        )}
        <button
          type="button"
          disabled={upload.isPending || disabled}
          onClick={() => fileInput.current?.click()}
          className="ml-auto inline-flex shrink-0 items-center gap-1.5 rounded-md border border-hairline-strong px-2 py-1 text-11 text-slate-300 transition-colors hover:bg-white/5 disabled:opacity-40 disabled:hover:bg-transparent"
        >
          <Upload size={12} className="shrink-0" />
          {upload.isPending ? "uploading…" : "Upload"}
        </button>
        <input
          ref={fileInput}
          type="file"
          accept=".ko,application/octet-stream"
          className="hidden"
          onChange={(e) => {
            send(e.target.files?.[0]);
            e.target.value = "";
          }}
        />
      </div>

      {upload.isError && (
        <Banner tone="error" className="m-3">
          {(upload.error as Error)?.message}
        </Banner>
      )}
      {upload.isSuccess && (
        <Banner tone="info" className="m-3">
          Uploaded - it appears below as pending until an admin approves it.
        </Banner>
      )}

      {isPending && <p className="px-3 py-3 text-13 text-slate-500">Loading catalog…</p>}
      {isError && (
        <p className="px-3 py-3 text-13 text-status-warn">Couldn’t load the driver catalog.</p>
      )}

      {!isPending && !isError && (
        <>
          {approved.length === 0 && mine.length === 0 ? (
            <p className="px-3 py-3 text-13 text-slate-500">
              No vetted drivers yet. Drop a <code className="text-slate-300">.ko</code> below, or
              build the sample driver in <code className="text-slate-300">/app/driver</code>.
            </p>
          ) : (
            // The pane is user-resizable from 390px to 1600px independently of
            // the viewport, so there is no viewport breakpoint that could hide
            // columns at the right moment. Rather than reach for a container
            // query (which needs CSS containment) or a ResizeObserver (which
            // does not fire in a non-rendering tab), every column simply stays
            // and the table scrolls on its own axis in the rare case someone
            // drags the pane to its minimum. Nothing to go wrong, and the page
            // itself never scrolls sideways.
            <div className="overflow-x-auto">
              <table className="w-full text-left">
                <thead>
                  <tr className="border-b border-hairline text-10 uppercase tracking-wider text-slate-600">
                    {/* Named via aria-label, NOT an `sr-only` span. Tailwind's
                        sr-only is `position: absolute`, and with no positioned
                        ancestor such a span escapes its cell, resolves against a
                        distant one, and lands past the fold - which extended the
                        document and made the whole PAGE scrollable. Wheel-scrolling
                        the guide then dragged the entire app upward. An aria-label
                        gives the column the same accessible name with no positioned
                        element to escape, so the bug cannot recur. */}
                    <th scope="col" aria-label="mount" className="w-8 px-3 py-1.5" />
                    <th scope="col" className="px-2 py-1.5 font-normal">module</th>
                    {expectedVermagic && (
                      <th scope="col" className="px-2 py-1.5 font-normal">kernel</th>
                    )}
                    <th scope="col" className="px-2 py-1.5 font-normal">license</th>
                    <th scope="col" className="px-2 py-1.5 text-right font-normal">size</th>
                  </tr>
                </thead>
                <tbody>
                  {approved.map((d) => {
                    const on = selected.includes(d.id);
                    const ok = fits(d);
                    const id = `${baseId}-${d.id}`;
                    return (
                      <tr
                        key={d.id}
                        // The whole row is a click target for convenience, but
                        // the checkbox owns its own change (so keyboard space
                        // still works), so ignore clicks that started there or
                        // the two handlers would toggle twice and cancel out.
                        onClick={(e) => {
                          if ((e.target as HTMLElement).closest("input")) return;
                          toggle(d.id);
                        }}
                        className={`border-b border-hairline/50 font-mono text-11 ${
                          on ? "bg-accent/10" : ""
                        } ${disabled ? "opacity-70" : "cursor-pointer hover:bg-white/5"}`}
                      >
                        <td className="px-3 py-1.5">
                          <input
                            id={id}
                            type="checkbox"
                            className="accent-accent"
                            checked={on}
                            disabled={disabled}
                            onChange={() => toggle(d.id)}
                          />
                        </td>
                        <td className="max-w-[10rem] px-2 py-1.5">
                          <label htmlFor={id} className="block truncate text-slate-200">
                            {d.modName || d.filename}
                          </label>
                          {d.description && (
                            <span className="block truncate font-sans text-10 text-slate-500">
                              {d.description}
                            </span>
                          )}
                        </td>
                        {expectedVermagic && (
                          <td className="px-2 py-1.5">
                            <span
                              className={`inline-flex items-center gap-1 ${
                                ok ? "text-status-ok" : "text-status-danger"
                              }`}
                              // The row already reads "mismatch"; spell out the
                              // consequence for anyone hovering, since the string
                              // itself means nothing without the comparison.
                              title={
                                ok
                                  ? `vermagic matches this lab's kernel (${d.vermagic.trim()})`
                                  : `built against ${d.vermagic.trim() || "an unknown kernel"}; insmod will reject it here`
                              }
                            >
                              {ok ? <Check size={11} /> : <AlertTriangle size={11} />}
                              {ok ? "match" : "mismatch"}
                            </span>
                          </td>
                        )}
                        <td className="px-2 py-1.5 text-slate-500">{d.license || "-"}</td>
                        <td className="whitespace-nowrap px-2 py-1.5 text-right text-slate-500">
                          {formatSize(d.sizeBytes)}
                        </td>
                      </tr>
                    );
                  })}

                  {mine.map((d) => (
                    <DriverStatusRows key={d.id} driver={d} />
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}

      {/* Upload is the only convenient way into an egress-denied guest, so it
          gets a real target rather than a link: a researcher who just ran `make`
          can drag the .ko straight in. */}
      <div
        onDragOver={(e) => {
          e.preventDefault();
          if (!disabled) setDragging(true);
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={(e) => {
          e.preventDefault();
          setDragging(false);
          send(e.dataTransfer.files?.[0]);
        }}
        className={`border-t border-hairline px-3 py-2.5 text-center text-10 transition-colors ${
          dragging ? "bg-accent/10 text-accent" : "text-slate-600"
        } ${disabled ? "opacity-50" : ""}`}
      >
        {disabled
          ? "Selection is locked once the session is running."
          : "Drop a .ko file to add it to the review queue"}
      </div>
    </div>
  );
}

// DriverStatusRows renders one of the caller's own unapproved uploads: a status
// line, plus the admin's reason on its own full-width row when there is one.
//
// The reason gets a row of its own rather than a truncated cell on purpose - it
// is the only channel telling a researcher why their module was rejected, and a
// clipped "Built against 6.12.1; the research gu…" sends them to support for
// something the payload already answered in full.
// colSpan is deliberately far larger than the column count: `license` and `size`
// are display:none below `sm`, and a colSpan counted from the LOGICAL columns
// then exceeds the rendered ones and stretches the table past its container -
// which clipped the reason text in a narrow lab panel. Browsers clamp an
// oversized colSpan to the columns actually present, so this is correct at every
// width.
const SPAN_ALL = 99;

function DriverStatusRows({ driver }: { driver: Driver }) {
  const denied = driver.status === "denied";
  return (
    <>
      <tr className="border-b border-hairline/50 font-mono text-11">
        <td className="px-3 py-1.5">
          {denied ? (
            <X size={11} className="text-status-danger" />
          ) : (
            <Clock size={11} className="text-status-warn" />
          )}
        </td>
        <td className="max-w-[10rem] px-2 py-1.5">
          <span className="block truncate text-slate-400">{driver.modName || driver.filename}</span>
        </td>
        <td colSpan={SPAN_ALL} className="px-2 py-1.5">
          <span className={denied ? "text-status-danger" : "text-status-warn"}>{driver.status}</span>
        </td>
      </tr>
      {driver.vetReason && (
        <tr className="border-b border-hairline/50">
          <td />
          <td colSpan={SPAN_ALL} className="px-2 pb-2 pr-3">
            <p className="font-sans text-10 leading-relaxed text-slate-500">{driver.vetReason}</p>
          </td>
        </tr>
      )}
    </>
  );
}

// formatSize keeps the column narrow: KB for anything under a megabyte, one
// decimal place, so the numbers stay comparable down the column.
function formatSize(bytes: number): string {
  if (!bytes) return "-";
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}
