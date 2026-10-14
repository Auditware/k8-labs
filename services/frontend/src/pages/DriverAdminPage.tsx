import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Download } from "lucide-react";
import {
  adminDriverDecision,
  adminListDrivers,
  driverDownloadUrl,
  type Driver,
} from "../lib/api";
import { Page, PageSection } from "../layout/Page";
import { Button } from "../components/ui/Form";
import { Banner } from "../components/ui/Banner";

// Admin -> Drivers.
//
// The vetting queue for the research driver catalog. A research user uploads a
// compiled .ko; it lands here as pending. An admin sees what the backend
// extracted from the ELF (name, vermagic, license, signed?, size, hash) plus who
// uploaded it, can download the raw module for out-of-band analysis, and then
// approves it (optionally into the shared catalog, visible to every research
// user) or denies it with a reason. See docs/architecture/research-driver-catalog.md.
//
// This is curation, not isolation control: loading a module inside the guest is
// already within the research threat model (the guest kernel is the target and
// the wall is the Firecracker microVM). Vetting keeps the shared catalog
// trustworthy and stops the platform hosting junk - it is not a load-time gate.

const STATUSES = ["pending", "approved", "denied"] as const;

function Row({ driver, status }: { driver: Driver; status: string }) {
  const qc = useQueryClient();
  const [catalog, setCatalog] = useState(true);
  const [err, setErr] = useState<string | null>(null);
  const invalidate = () => qc.invalidateQueries({ queryKey: ["admin", "drivers"] });

  const decide = useMutation({
    mutationFn: (v: { decision: "approve" | "deny"; catalog?: boolean; reason?: string }) =>
      adminDriverDecision(driver.id, v.decision, { catalog: v.catalog, reason: v.reason }),
    onSuccess: () => {
      setErr(null);
      invalidate();
    },
    onError: (e: Error) => setErr(e.message),
  });

  const deny = () => {
    const reason = window.prompt("Reason for denying this driver? (shown to the uploader)");
    if (reason === null) return; // cancelled
    decide.mutate({ decision: "deny", reason });
  };

  return (
    <tr className="border-t border-slate-800 align-top">
      <td className="py-3 pr-4">
        <div className="font-mono text-13 text-slate-200">{driver.modName || driver.filename}</div>
        <div className="mt-0.5 truncate font-mono text-10 text-slate-500" title={driver.vermagic}>
          {driver.vermagic || "vermagic unknown"}
        </div>
        <div className="mt-0.5 flex flex-wrap items-center gap-1.5">
          {driver.license && (
            <span className="font-mono text-10 text-slate-500">{driver.license}</span>
          )}
          <span
            className={`rounded-full px-1.5 py-0.5 font-mono text-10 uppercase tracking-wider ring-1 ring-inset ${
              driver.isSigned
                ? "bg-emerald-500/10 text-emerald-400 ring-emerald-500/30"
                : "bg-slate-700/20 text-slate-500 ring-slate-700"
            }`}
          >
            {driver.isSigned ? "signed" : "unsigned"}
          </span>
          {status === "approved" && driver.visibility === "catalog" && (
            <span className="rounded-full bg-blue-500/10 px-1.5 py-0.5 font-mono text-10 uppercase tracking-wider text-blue-400 ring-1 ring-inset ring-blue-500/30">
              catalog
            </span>
          )}
          {status === "denied" && driver.vetReason && (
            <span className="text-10 text-red-400" title={driver.vetReason}>
              denied: {driver.vetReason}
            </span>
          )}
        </div>
      </td>
      <td className="py-3 pr-4 font-mono text-11 text-slate-500">
        {(driver.sizeBytes / 1024).toFixed(1)} KB
        <div className="truncate" title={driver.sha256}>
          {driver.sha256.slice(0, 12)}
        </div>
      </td>
      <td className="py-3 pr-4 text-11 text-slate-500">
        {driver.uploadedBy}
        <div>{new Date(driver.uploadedAt).toLocaleDateString()}</div>
      </td>
      <td className="py-3 text-right">
        <div className="flex flex-wrap items-center justify-end gap-2">
          {err && <span className="max-w-xs text-left text-11 text-red-400">{err}</span>}
          <a
            href={driverDownloadUrl(driver.id)}
            className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-11 text-slate-400 ring-1 ring-inset ring-slate-700 hover:bg-slate-800 hover:text-slate-200"
            title="Download the raw .ko for out-of-band analysis"
          >
            <Download size={13} /> .ko
          </a>
          {status === "pending" && (
            <>
              <label className="flex items-center gap-1 text-11 text-slate-400" title="Make it visible to every research user, not just the uploader">
                <input
                  type="checkbox"
                  className="accent-blue-500"
                  checked={catalog}
                  onChange={(e) => setCatalog(e.target.checked)}
                />
                catalog
              </label>
              <Button
                size="sm"
                onClick={() => decide.mutate({ decision: "approve", catalog })}
                disabled={decide.isPending}
              >
                Approve
              </Button>
              <Button variant="danger" size="sm" onClick={deny} disabled={decide.isPending}>
                Deny
              </Button>
            </>
          )}
          {status !== "pending" && (
            <Button variant="ghost" size="sm" onClick={deny} disabled={decide.isPending}>
              {status === "approved" ? "Revoke" : "Re-deny"}
            </Button>
          )}
        </div>
      </td>
    </tr>
  );
}

export default function DriverAdminPage() {
  const [status, setStatus] = useState<(typeof STATUSES)[number]>("pending");
  const { data, isPending, isError, error } = useQuery({
    queryKey: ["admin", "drivers", status],
    queryFn: () => adminListDrivers(status),
  });

  return (
    <Page title="Drivers">
      <PageSection title="Research driver vetting">
        <div className="mb-4 flex gap-1.5">
          {STATUSES.map((s) => (
            <Button
              key={s}
              size="sm"
              variant={s === status ? "primary" : "ghost"}
              onClick={() => setStatus(s)}
            >
              {s[0].toUpperCase() + s.slice(1)}
            </Button>
          ))}
        </div>

        {isPending && !isError && <p className="text-sm text-slate-600">Loading…</p>}
        {isError && (
          <Banner tone="error">
            Failed to load drivers{error instanceof Error ? `: ${error.message}` : ""}
          </Banner>
        )}

        {data && data.length > 0 && (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[46rem] text-left">
              <thead>
                <tr className="text-11 uppercase tracking-wide text-slate-500">
                  <th className="pb-2 font-medium">Module</th>
                  <th className="pb-2 font-medium">Size / hash</th>
                  <th className="pb-2 font-medium">Uploaded by</th>
                  <th className="pb-2" />
                </tr>
              </thead>
              <tbody>
                {data.map((d) => (
                  <Row key={d.id} driver={d} status={status} />
                ))}
              </tbody>
            </table>
          </div>
        )}
        {data && data.length === 0 && (
          <p className="text-sm text-slate-600">Nothing {status}.</p>
        )}
      </PageSection>
    </Page>
  );
}
