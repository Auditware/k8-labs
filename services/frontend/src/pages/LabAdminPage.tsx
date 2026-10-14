import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { getLabSettings, resetLabSetting, setLabSetting, type LabSetting } from "../lib/api";
import { Page, PageSection } from "../layout/Page";
import { Button } from "../components/ui/Form";

// Admin -> Labs.
//
// Operational knobs only. Lesson content is NOT editable here and never will
// be: it lives in git, is reviewed in a PR, and ships embedded in the backend
// build (see docs/architecture/lab-session-rewrite.md, decision 6). The line is
// content in git, operational settings in the database - so this page manages
// how long a session lasts, not what it teaches.
//
// These values are not cosmetic. duration_minutes is what the lab UI counts
// down to and what the reaper enforces, so a change here changes how long a
// student's work survives. Sessions already running keep the timing they
// started with: it is resolved once at create time (internal/labs/timing.go),
// so editing a lab here never shortens a session someone is in the middle of.
//
// None of that is spelled out on the page. It was, in a subtitle and a callout
// box, and it dwarfed the two number fields it explained - the table's own
// column headers already say Duration and Idle timeout, which is the part an
// admin actually needs at the moment they are editing.

function Row({ setting }: { setting: LabSetting }) {
  const qc = useQueryClient();
  const [duration, setDuration] = useState(String(setting.durationMinutes));
  const [idle, setIdle] = useState(String(setting.idleMinutes));
  const [err, setErr] = useState<string | null>(null);

  const dirty =
    Number(duration) !== setting.durationMinutes || Number(idle) !== setting.idleMinutes;

  const invalidate = () => qc.invalidateQueries({ queryKey: ["lab-settings"] });

  const save = useMutation({
    mutationFn: () => setLabSetting(setting.moduleSlug, Number(duration), Number(idle)),
    onSuccess: () => {
      setErr(null);
      invalidate();
    },
    // The API answers with the actual reason ("idle timeout cannot exceed the
    // session duration"), which is far more useful to an admin than a status
    // code - so surface it rather than a generic failure.
    onError: (e: Error) => setErr(e.message),
  });

  const reset = useMutation({
    mutationFn: () => resetLabSetting(setting.moduleSlug),
    onSuccess: () => {
      setErr(null);
      setDuration(String(60));
      setIdle(String(20));
      invalidate();
    },
    onError: (e: Error) => setErr(e.message),
  });

  return (
    <tr className="border-t border-slate-800">
      <td className="py-3 pr-4">
        <div className="font-mono text-13 text-slate-200">{setting.moduleSlug}</div>
        {setting.isDefault ? (
          // Saying "default" rather than showing the numbers as if someone
          // chose them - otherwise an admin can't tell configured from
          // untouched.
          <div className="text-11 text-slate-600">using the built-in default</div>
        ) : (
          <div className="text-11 text-slate-500">
            customised{setting.updatedAt ? ` · ${new Date(setting.updatedAt).toLocaleDateString()}` : ""}
          </div>
        )}
      </td>
      <td className="py-3 pr-3">
        <input
          type="number"
          value={duration}
          onChange={(e) => setDuration(e.target.value)}
          className="w-20 rounded-md border border-slate-800 bg-slate-950 px-2 py-1 text-right font-mono text-13 text-slate-200 focus:border-blue-500 focus:outline-none"
          aria-label={`Session duration for ${setting.moduleSlug}, in minutes`}
        />
        <span className="ml-1.5 text-11 text-slate-600">min</span>
      </td>
      <td className="py-3 pr-3">
        <input
          type="number"
          value={idle}
          onChange={(e) => setIdle(e.target.value)}
          className="w-20 rounded-md border border-slate-800 bg-slate-950 px-2 py-1 text-right font-mono text-13 text-slate-200 focus:border-blue-500 focus:outline-none"
          aria-label={`Idle timeout for ${setting.moduleSlug}, in minutes`}
        />
        <span className="ml-1.5 text-11 text-slate-600">min</span>
      </td>
      <td className="py-3 text-right">
        <div className="flex items-center justify-end gap-2">
          {err && <span className="max-w-xs text-left text-11 text-red-400">{err}</span>}
          <Button size="sm" onClick={() => save.mutate()} disabled={!dirty || save.isPending}>
            {save.isPending ? "Saving…" : "Save"}
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => reset.mutate()}
            disabled={setting.isDefault || reset.isPending}
            title="Drop the override so this lab uses the built-in default"
          >
            Reset
          </Button>
        </div>
      </td>
    </tr>
  );
}

export default function LabAdminPage() {
  // isPending, not isLoading. Between retry attempts react-query leaves
  // isLoading false while isError is still unset, so an isLoading/isError pair
  // renders a blank page for the whole backoff - which is exactly when an admin
  // most needs to be told something is happening.
  const { data, isPending, isError, error } = useQuery({
    queryKey: ["lab-settings"],
    queryFn: getLabSettings,
  });

  return (
    <Page title="Labs">
      <PageSection title="Session limits">
        {isPending && !isError && <p className="text-sm text-slate-600">Loading…</p>}
        {isError && (
          <p className="text-sm text-red-400">
            Failed to load lab settings{error instanceof Error ? `: ${error.message}` : ""}
          </p>
        )}

        {data && data.length > 0 && (
          <table className="w-full text-left">
            <thead>
              <tr className="text-11 uppercase tracking-wide text-slate-500">
                <th className="pb-2 font-medium">Lab</th>
                <th className="pb-2 font-medium">Duration</th>
                <th className="pb-2 font-medium">Idle timeout</th>
                <th className="pb-2" />
              </tr>
            </thead>
            <tbody>
              {data.map((s) => (
                // Keyed by slug AND its current values so the inputs reset to
                // the server's answer after a save or reset, instead of holding
                // stale local state.
                <Row key={`${s.moduleSlug}:${s.durationMinutes}:${s.idleMinutes}`} setting={s} />
              ))}
            </tbody>
          </table>
        )}
        {data && data.length === 0 && <p className="text-sm text-slate-600">No labs in the catalog.</p>}
      </PageSection>
    </Page>
  );
}
