import { useEffect, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Copy, ExternalLink, KeyRound, Trash2, UserRound } from "lucide-react";
import {
  createApiKey,
  deleteApiKey,
  deleteAvatar,
  getApiKeys,
  getProfile,
  listDrivers,
  updateProfile,
  updateUsername,
  uploadAvatar,
  SCOPE_DRIVERS_WRITE,
  type ApiKey,
} from "../lib/api";
import { useAuth } from "../context/AuthContext";
import { Page, PageSection } from "../layout/Page";
import { Button, Field, Input, Select, Textarea } from "../components/ui/Form";
import { Card } from "../components/ui/Card";
import { Banner } from "../components/ui/Banner";
import { AvatarPicker } from "../components/ui/AvatarPicker";
import { useAutosave } from "../lib/useAutosave";

// Settings -> General. Everything here acts on the caller's own account; the
// backend takes the user from the session, never from a path parameter, so
// there's no "edit someone else" surface to guard.
export default function SettingsPage() {
  const { user, avatarVersion, refreshUser } = useAuth();

  const profileQuery = useQuery({ queryKey: ["profile"], queryFn: getProfile });
  const keysQuery = useQuery({ queryKey: ["apikeys"], queryFn: getApiKeys });

  const [website, setWebsite] = useState("");
  const [bio, setBio] = useState("");
  const [x, setX] = useState("");
  const [linkedin, setLinkedin] = useState("");

  const saveProfile = useMutation({
    mutationFn: (v: { website: string; bio: string; x: string; linkedin: string }) =>
      updateProfile(v.website, v.bio, v.x, v.linkedin),
    // No invalidate on success. The server has what we sent, and refetching
    // would push the values back into the inputs the user is still editing.
    // A failure surfaces below; that is the only feedback this form gives.
  });

  // Autosaved, so there is no Save button and no "Saved" flash. See
  // lib/useAutosave for why the first render can't trigger a write.
  const autosave = useAutosave({ website, bio, x, linkedin }, (v) => saveProfile.mutate(v));

  // Seed the form once the profile lands. sync() first, so the values we're
  // about to put in the inputs are recorded as "what the server holds" and
  // populating the form isn't mistaken for an edit.
  useEffect(() => {
    const p = profileQuery.data;
    if (p) {
      autosave.sync({ website: p.website, bio: p.bio, x: p.x, linkedin: p.linkedin });
      setWebsite(p.website);
      setBio(p.bio);
      setX(p.x);
      setLinkedin(p.linkedin);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [profileQuery.data]);

  // Both re-read the auth user, which is what carries hasAvatar to the whole app
  // (nav included) and bumps avatarVersion to bust the image cache - so there's
  // no separate ["profile"] invalidation: avatar presence no longer rides on
  // that query.
  const upload = useMutation({
    mutationFn: (file: File) => uploadAvatar(file),
    onSuccess: () => refreshUser(),
  });

  const removeAvatar = useMutation({
    mutationFn: () => deleteAvatar(),
    onSuccess: () => refreshUser(),
  });

  const profileError = saveProfile.error || upload.error || removeAvatar.error;

  return (
    <Page title="Settings">
      <PageSection title="Profile" icon={<UserRound size={13} />}>
        <Card>
          {profileError && (
            <Banner tone="error" className="mb-5">
              {(profileError as Error).message}
            </Banner>
          )}

          <div className="flex flex-col gap-6 sm:flex-row sm:items-start">
            {user && (
              <AvatarPicker
                user={user}
                version={avatarVersion}
                onPick={(f) => upload.mutate(f)}
                onRemove={() => removeAvatar.mutate()}
                busy={upload.isPending || removeAvatar.isPending}
              />
            )}

            <div className="flex-1 space-y-5">
              <DisplayNameField />

              <Field label="Website">
                <Input
                  type="url"
                  value={website}
                  onChange={(e) => setWebsite(e.target.value)}
                  placeholder="https://example.com"
                />
              </Field>

              <Field label="Bio">
                <Textarea
                  value={bio}
                  onChange={(e) => setBio(e.target.value)}
                  rows={3}
                  maxLength={1000}
                  placeholder="A short description."
                />
              </Field>

              <div className="grid gap-5 sm:grid-cols-2">
                <Field label="X">
                  <Input
                    type="url"
                    value={x}
                    onChange={(e) => setX(e.target.value)}
                    placeholder="https://x.com/yourhandle"
                  />
                </Field>
                <Field label="LinkedIn">
                  <Input
                    type="url"
                    value={linkedin}
                    onChange={(e) => setLinkedin(e.target.value)}
                    placeholder="https://linkedin.com/in/you"
                  />
                </Field>
              </div>
            </div>
          </div>
        </Card>
      </PageSection>

      <ApiKeysSection keys={keysQuery.data ?? []} isLoading={keysQuery.isLoading} />
    </Page>
  );
}

// The display name is the username handle, which lives on the auth user rather
// than the autosaved Profile - so it gets an explicit Save (claiming a unique
// public handle shouldn't fire on every keystroke) and refreshes the auth
// context on success so the nav and public-profile URL update everywhere.
function DisplayNameField() {
  const { user, refreshUser } = useAuth();
  const [name, setName] = useState("");

  // Seed once from the auth user; after that the field is the user's to edit,
  // and a successful save re-seeds it from the stored value below.
  const seeded = useRef(false);
  useEffect(() => {
    if (!seeded.current && user?.username) {
      setName(user.username);
      seeded.current = true;
    }
  }, [user?.username]);

  const save = useMutation({
    mutationFn: (v: string) => updateUsername(v),
    onSuccess: (res) => {
      setName(res.username);
      refreshUser();
    },
  });

  const trimmed = name.trim();
  const changed = trimmed !== "" && trimmed !== (user?.username ?? "");

  return (
    <div>
      <span className="mb-1.5 block text-13 font-medium text-slate-300">Display name</span>
      <div className="flex items-center gap-2">
        <Input
          value={name}
          onChange={(e) => setName(e.target.value)}
          maxLength={32}
          spellCheck={false}
          autoComplete="off"
          className="flex-1"
          onKeyDown={(e) => {
            if (e.key === "Enter" && changed && !save.isPending) save.mutate(trimmed);
          }}
        />
        <Button onClick={() => save.mutate(trimmed)} disabled={!changed || save.isPending}>
          {save.isPending ? "Saving…" : "Save"}
        </Button>
      </div>
      {save.error && (
        <span className="mt-1.5 block text-xs text-red-400">{(save.error as Error).message}</span>
      )}
      <span className="mt-1.5 block text-xs text-slate-500">
        Shown on the leaderboard and in your public profile URL.
      </span>
    </div>
  );
}

// A suggested name, so the field is never empty and the user never has to
// invent one. Overwriting it is the whole point - it is a default, not a
// placeholder, so it submits as-is if they don't care.
function suggestKeyName(): string {
  return `my_agent_${crypto.randomUUID().replace(/-/g, "").slice(0, 6)}`;
}

function ApiKeysSection({ keys, isLoading }: { keys: ApiKey[]; isLoading: boolean }) {
  const queryClient = useQueryClient();
  const { user } = useAuth();
  const [name, setName] = useState(suggestKeyName);
  const [expiresInDays, setExpiresInDays] = useState(30);
  const [driversWrite, setDriversWrite] = useState(false);
  // Held in component state only, never persisted: this is the one moment the
  // plaintext key exists client-side, and it's gone on navigation by design.
  const [freshKey, setFreshKey] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  // Whether this user may mint a drivers:write key. The driver catalog list is
  // gated by the exact same research-access check the backend uses for the
  // scope, so a successful call is the authoritative "you're entitled" signal -
  // no separate endpoint needed. Admins pass it too. Failures (403) just hide
  // the option; the backend still enforces on create either way.
  const canScopeDrivers = useQuery({
    queryKey: ["research-access"],
    queryFn: listDrivers,
    retry: false,
    staleTime: 5 * 60 * 1000,
  }).isSuccess;

  const create = useMutation({
    mutationFn: () =>
      createApiKey(name, expiresInDays, driversWrite ? [SCOPE_DRIVERS_WRITE] : []),
    onSuccess: (res) => {
      setFreshKey(res.key);
      // A fresh suggestion, not an empty box - the next key is as easy to
      // make as the first.
      setName(suggestKeyName());
      setDriversWrite(false);
      queryClient.invalidateQueries({ queryKey: ["apikeys"] });
    },
  });

  const revoke = useMutation({
    mutationFn: (id: string) => deleteApiKey(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["apikeys"] }),
  });

  const isAdmin = user?.role === "admin";

  return (
    <PageSection title="API keys" icon={<KeyRound size={13} />}>
      <Card>
        {(create.error || revoke.error) && (
          <Banner tone="error" className="mb-5">
            {((create.error || revoke.error) as Error).message}
          </Banner>
        )}

        {freshKey && (
          <div className="mb-5 rounded-lg border border-amber-500/30 bg-amber-500/10 p-4">
            <p className="mb-2.5 text-13 font-medium text-amber-200">
              Copy this now - it won't be shown again.
            </p>
            <div className="flex items-center gap-2">
              <code className="flex-1 overflow-x-auto rounded-lg bg-slate-950 px-3 py-2.5 text-13 text-amber-100">
                {freshKey}
              </code>
              <Button
                variant="ghost"
                onClick={() => {
                  navigator.clipboard?.writeText(freshKey);
                  setCopied(true);
                  setTimeout(() => setCopied(false), 1500);
                }}
                className="!text-amber-200 !ring-amber-500/30 hover:!bg-amber-500/10"
              >
                <Copy size={14} /> {copied ? "Copied" : "Copy"}
              </Button>
            </div>
            <button
              type="button"
              onClick={() => setFreshKey(null)}
              className="mt-2.5 text-13 text-amber-300/70 underline hover:text-amber-200"
            >
              Dismiss
            </button>
          </div>
        )}

        <div className="mb-6 flex flex-wrap items-end gap-3">
          <div className="min-w-[14rem] flex-1">
            <Field label="Name">
              <Input type="text" value={name} onChange={(e) => setName(e.target.value)} />
            </Field>
          </div>
          <div className="w-40">
            <Field label="Expires">
              <Select
                value={expiresInDays}
                onChange={(e) => setExpiresInDays(Number(e.target.value))}
              >
                <option value={7}>7 days</option>
                <option value={30}>30 days</option>
                <option value={90}>90 days</option>
                <option value={365}>1 year</option>
                <option value={0}>Never</option>
              </Select>
            </Field>
          </div>
          <Button onClick={() => create.mutate()} disabled={create.isPending || !name.trim()}>
            {create.isPending ? "Creating…" : "Create key"}
          </Button>
        </div>

        {canScopeDrivers && (
          <label className="mb-6 -mt-2 flex max-w-xl cursor-pointer items-start gap-2.5 text-13 text-slate-400">
            <input
              type="checkbox"
              className="mt-0.5 accent-blue-500"
              checked={driversWrite}
              onChange={(e) => setDriversWrite(e.target.checked)}
            />
            <span>
              Allow uploading research drivers
              <code className="ml-1.5 rounded bg-slate-800 px-1 py-0.5 font-mono text-11 text-slate-300">
                {SCOPE_DRIVERS_WRITE}
              </code>
              <span className="mt-0.5 block text-slate-500">
                Lets an agent using this key upload compiled <code className="text-slate-400">.ko</code>{" "}
                modules to the research driver catalog. Leave off for a general-purpose key.
              </span>
            </span>
          </label>
        )}

        {isLoading ? (
          <p className="text-sm text-slate-500">Loading keys…</p>
        ) : keys.length === 0 ? (
          <p className="text-sm text-slate-600">No API keys yet.</p>
        ) : (
          <div className="overflow-x-auto rounded-lg border border-slate-800">
            <table className="w-full min-w-[34rem] text-left text-sm">
              <thead className="bg-slate-900/60 text-xs uppercase tracking-wide text-slate-500">
                <tr>
                  <th className="px-4 py-2.5 font-semibold">Name</th>
                  <th className="px-4 py-2.5 font-semibold">Key</th>
                  <th className="px-4 py-2.5 font-semibold">Expires</th>
                  <th className="px-4 py-2.5 font-semibold">Last used</th>
                  <th className="px-4 py-2.5" />
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800">
                {keys.map((k) => (
                  <tr key={k.id}>
                    <td className="px-4 py-3 text-slate-200">
                      {k.name}
                      {k.scopes?.length > 0 && (
                        <span className="mt-1 flex flex-wrap gap-1">
                          {k.scopes.map((s) => (
                            <span
                              key={s}
                              className="rounded-full bg-blue-500/10 px-1.5 py-0.5 font-mono text-10 text-blue-400 ring-1 ring-inset ring-blue-500/30"
                            >
                              {s}
                            </span>
                          ))}
                        </span>
                      )}
                    </td>
                    <td className="px-4 py-3">
                      <code className="text-13 text-slate-500">{k.prefix}…</code>
                    </td>
                    <td className="px-4 py-3 text-13 text-slate-500">
                      {k.expiresAt ? formatDate(k.expiresAt) : "Never"}
                    </td>
                    <td className="px-4 py-3 text-13 text-slate-500">
                      {k.lastUsedAt ? formatDate(k.lastUsedAt) : "Never"}
                    </td>
                    <td className="px-4 py-3 text-right">
                      <Button
                        variant="danger"
                        size="sm"
                        onClick={() => revoke.mutate(k.id)}
                        disabled={revoke.isPending}
                        title="Revoke this key"
                      >
                        <Trash2 size={13} /> Revoke
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        <div className="mt-6 border-t border-slate-800 pt-4">
          <a
            href="/swagger/"
            target="_blank"
            rel="noreferrer"
            className="inline-flex items-center gap-1.5 text-13 text-blue-400 hover:text-blue-300"
          >
            <ExternalLink size={14} /> API documentation
          </a>
          <p className="mt-1 text-xs text-slate-600">
            {isAdmin
              ? "Your docs include the admin endpoints."
              : "Scoped to the endpoints your account can call."}
          </p>
        </div>
      </Card>
    </PageSection>
  );
}

function formatDate(iso: string): string {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? "-" : d.toLocaleDateString();
}
