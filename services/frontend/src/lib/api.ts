// Relative /api path - Traefik routes it to the Backend in every
// environment, so this file never needs a build-time base URL. See
// docs/architecture/environments.md.

export type Module = {
  slug: string;
  displayName: string;
  description: string;
  hint: string;
  // Every module is nested-shape: its session is a disposable k3s cluster and
  // the terminal is an exec-based xterm, never a ttyd iframe (see
  // internal/labs/registry.go's NestedSpec and
  // docs/architecture/lab-session-rewrite.md, F1). Optional only because the
  // backend omits it when EntryPort is zero.
  nested?: { entryPort: number };
  // eBPF modules run an in-guest Tetragon sensor the student can watch in the
  // Defender pane (see internal/labs/registry.go's Module.EBPF and
  // docs/architecture/ebpf-per-session.md). Absent/false for every other lab.
  ebpf?: boolean;
  // Whether this module ships a Defender pane (an in-guest sensor stream to
  // tail). Drives the Defender tab. Distinct from `ebpf`: the research labs have
  // a Defender pane but select their kernel via `kernel`, not `ebpf`, so the tab
  // is gated on this, not on ebpf (backend: Module.HasDefender, json "defender").
  defender?: boolean;
  // Whether the calling user has solved (captured the flag for) this lab. Set
  // per-caller by the backend's listModules; drives the sidebar's solved marker
  // and the "N cleared" progress. Absent is treated as false.
  completed?: boolean;
  // The tactic bucket the sidebar groups this lab under (first-flight, recon,
  // privesc, lateral, escape, persistence). Set by the backend from its central
  // trackOf map (see internal/labs/registry.go). The sidebar renders the catalog
  // as track sections in a fixed order rather than one flat list.
  track?: string;
  // The vermagic a .ko must carry to insmod into this lab's guest kernel. Only
  // the research labs mount user drivers, so only they set it. DriverPicker
  // compares each driver's vermagic against this to flag an unloadable module
  // before the session starts; when absent it hides the compatibility column
  // rather than guessing (backend: Module.ExpectedVermagic).
  expectedVermagic?: string;
};

export type Session = {
  id: string;
  moduleSlug: string;
  status: string;
  error?: string;
  // phase is the last provisioning gate cleared while status is "starting" (see
  // internal/labs provisionNested: "pod-created" | "pod-ready" |
  // "kubeconfig-fetched" | "nested-node-ready" | "manifests-applied" |
  // "foothold-ready"). Absent until the first gate clears, and on legacy modules
  // that come back "running" immediately. ProvisioningLoader anchors its progress
  // bar to this so a slow boot can't run the bar ahead of reality; when it's
  // missing the loader falls back to its time-based estimate.
  phase?: string;
  // Both deadlines are computed server-side from the session's own limits (see
  // internal/labs/timing.go) - never from a locally remembered start time. A
  // client-side countdown drifts, doesn't survive a device change, and is
  // trivially editable, none of which is acceptable for a number that tells the
  // student when their work disappears.
  //
  // expiresAt is the hard cap, fixed at create time - the promise ("this lab is
  // 60 minutes"). idleExpiresAt slides forward on every keystroke and is what
  // actually reaps someone who is reading rather than typing, so the UI has to
  // surface it separately: showing only expiresAt would let a session vanish
  // with half an hour apparently left on the clock.
  expiresAt: string;
  idleExpiresAt: string;
  // Present only when status is "ended" - a tombstone the backend serves for a
  // short window after a session is torn down (see internal/labs EndedSession),
  // in place of the old bare 404. reason is why it stopped ("idle" reaped after
  // inactivity, "expired" hit its time limit, "stopped" ended explicitly), and
  // idleSeconds/maxAgeSeconds are the limits it ran under so the UI can name the
  // real timeout ("auto-closed after 10 min of inactivity") rather than a
  // featureless "this session ended". Live sessions omit these.
  reason?: string;
  endedAt?: string;
  idleSeconds?: number;
  maxAgeSeconds?: number;
};

// --- Admin -> Lab Management (see internal/api/adminlabs.go). ---

export type LabSetting = {
  moduleSlug: string;
  durationMinutes: number;
  idleMinutes: number;
  // True when no admin has overridden this lab - the values shown are the
  // built-in defaults, not somebody's choice.
  isDefault: boolean;
  updatedAt?: string;
  updatedBy?: string;
};

// --- Admin - see internal/health and internal/api/routes.go. ---

export type AdminPod = {
  name: string;
  ready: string;
  phase: string;
  restarts: number;
};

export type AdminNamespace = {
  name: string;
  pods: AdminPod[];
};

// --- Admin topology map - see internal/topology. ---

export type TopoNode = {
  name: string;
  role: string; // "control-plane" | "labs" | "worker"
  ready: boolean;
  kernel: string;
  runtime: string;
  os: string;
  // Present on every live payload; optional in the type so test/mock node
  // literals need not restate them. The capacity charts read the summed values
  // off the computed `capacity` block, not off individual nodes.
  allocatableCpuMillis?: number;
  allocatableMemBytes?: number;
};

export type TopoPod = {
  name: string;
  app?: string; // app.kubernetes.io/name - the stable edge-matching key
  workload: string;
  namespace: string;
  node?: string;
  phase: string;
  ready: string;
  restarts: number;
  ip?: string;
  containers: string[];
};

export type NsResources = {
  secrets?: number;
  configMaps?: number;
  services?: number;
  serviceAccounts?: number;
  roles?: number;
  roleBindings?: number;
  networkPolicies?: number;
  secretNames?: string[];
};
export type TopoNamespace = { name: string; pods: TopoPod[]; resources?: NsResources };
export type TopoEdge = { from: string; to: string; port?: string; kind: string };
export type TopoLayer = { nodeRole: string; icon?: string; title: string; sub?: string; kind?: string };
export type TopoDashboard = { ref: string; label: string; path: string };
export type NestedCluster = { reachable: boolean; namespaces?: TopoNamespace[]; edges?: TopoEdge[]; error?: string };
export type TopoSession = { id: string; module: string; status: string; cluster?: NestedCluster };

export type ArgoSignal = { synced: number; total: number; outOfSync?: string[] };
export type CertSignal = { soonestDays: number; expiring?: string[] };
export type Signals = { argo?: ArgoSignal; certs?: CertSignal };

// One user's slice of the reserved totals. user === "" is the folded "others"
// row. See topology.CapacityUser.
export type CapacityUser = {
  user: string;
  memBytes: number;
  cpuMillis: number;
  sessions: number;
};

// Labs-node resource rollup for the capacity charts. All scheduler-visible
// reservations vs allocatable, not live host telemetry - see topology.Capacity.
// byUser attributes the reserved totals to the users holding them (heaviest
// first, tail folded into "others"); the parts sum to the totals.
export type Capacity = {
  memReservedBytes: number;
  memBudgetBytes: number;
  memAllocatableBytes: number;
  // Non-lab platform baseline on the labs node (tetragon, CNI, coredns, ...),
  // drawn as a muted base under the lab usage so a column shows honest headroom.
  memSystemReservedBytes: number;
  cpuSystemReservedMillis: number;
  cpuReservedMillis: number;
  cpuAllocatableMillis: number;
  byUser?: CapacityUser[];
};

export type AdminTopology = {
  nodes: TopoNode[];
  namespaces: TopoNamespace[];
  edges: TopoEdge[];
  layers: TopoLayer[];
  dashboards: TopoDashboard[];
  sessions: TopoSession[];
  signals?: Signals;
  capacity?: Capacity;
};

export type User = {
  id: string;
  email: string;
  username: string;
  oauthProvider: string;
  role: string;
  blocked: boolean;
  // `avatar IS NOT NULL` server-side (see accounts.User). Lets any component
  // with the auth user decide between the uploaded image at avatarUrl(id) and
  // an initial fallback without separately fetching the profile - which is why
  // <Avatar> can live in the nav, not just on the settings page.
  hasAvatar: boolean;
};

// A user's resource consumption. See internal/accounts.UserUsage.
//
// The components arrive un-blended on purpose: a single "credits" figure would
// bake one exchange rate into stored history, and the weights are exactly what
// there is no data to choose yet. Any score is computed here, from these.
//
// gibMinutes is memory reserved x wall-clock minutes, which is what a session
// actually denies to everyone else - it holds its whole reservation whether or
// not the student is typing. An eBPF lab therefore costs ~1.67x a normal one per
// minute. cpuSeconds and egressBytes are reported but not yet populated (they
// need collectors that do not exist yet); they are here so the shape is stable.
export type UserUsage = {
  userId: string;
  windowGibMinutes: number;
  windowSessions: number;
  totalGibMinutes: number;
  totalSessions: number;
  totalCpuSeconds: number;
  totalEgressBytes: number;
  lastSessionAt?: string;
};

// A user's IN-FLIGHT consumption: sessions running right now. The live
// complement to UserUsage, which only counts finished sessions. gibMinutes is
// accrued-so-far, so it grows smoothly into the value the finished row will
// eventually record. See internal/labs.LiveUsage.
export type LiveUsage = {
  sessions: number;
  reservedBytes: number;
  gibMinutes: number;
};

// The admin user-management view: an account plus the module slugs it's been
// granted, plus consumption. See internal/accounts.AdminUser.
//
// usage is HISTORICAL (finished sessions) and absent for an account that has
// never completed one - different from one whose sessions cost zero, so the UI
// shows a dash rather than a 0. live is what the user is consuming this instant,
// absent when they have no running session. A student mid-lab has live but not
// yet usage, which is exactly the case that used to read as "-".
export type AdminUser = User & {
  grants: string[];
  // Module slugs this user has solved (submitted the captured flag). Distinct
  // from grants (what they can access): the grant menu marks which granted labs
  // are actually cleared. Always present, [] for a user who has solved nothing.
  completed: string[];
  // Module slugs this user can actually ACCESS: grants folded in with the free
  // progression unlocks (tier-0 for everyone, plus each tier the previous one
  // unlocked). The lab-access summary counts this, not grants, so a user on the
  // free first lab doesn't read as "No labs". Always present.
  entitled: string[];
  usage?: UserUsage;
  live?: LiveUsage;
  // Per-user daily quota override in GiB-minutes, absent when the user falls
  // back to the global default. 0 means unlimited (exempt). Admin-editable.
  quotaOverrideGibMinutes?: number;
  // When this user most recently captured a flag (ISO timestamp), absent for a
  // user who has never solved a lab. The admin list is sorted by this (most
  // recent first), and the row shows it as a "last solved" hint.
  lastSolvedAt?: string;
};

// --- Auth is the httpOnly "token" cookie, nothing else.
//
// The backend also returns the raw JWT in the login response body, and this
// file used to mirror it into localStorage and send it as a Bearer header.
// That's been dropped: the cookie was always required anyway (the ttyd
// terminal iframe and its websocket can't attach a custom Authorization
// header at all), so the localStorage copy was a second, strictly weaker
// carrier of the same credential - readable by any XSS on the page, which in
// a product whose whole purpose is running exploit code in the browser is a
// bad trade. The cookie is httpOnly + SameSite=Strict + Secure over HTTPS
// (see internal/api/auth.go's authCookie), so script can't read it and
// cross-site requests can't ride it.
//
// Every call below therefore sends credentials: "include" and nothing else.
// requireAuth accepts either carrier, so this needed no backend change. ---

// One-time cleanup of the key the Bearer-token version of this file wrote.
// Without this, anyone who signed in before the switch keeps a live JWT in
// localStorage until it expires (a week, see sessionTTL) - exactly the
// XSS-readable copy dropping the Bearer path was meant to eliminate. Runs at
// module load so it happens on the next page view, not the next logout.
// Safe to delete once no session predating the change can still be live.
// Guarded so this never runs during server-side rendering, where `localStorage`
// is undefined. (The try still covers private-mode/blocked storage in the browser.)
if (typeof window !== "undefined") {
  try {
    localStorage.removeItem("auth_token");
  } catch {
    // Private-mode/blocked storage - nothing to clean up in that case anyway.
  }
}

export type ModulesResult = {
  // The modules the caller can see (entitlement-filtered server-side).
  modules: Module[];
  // How many catalog labs are still locked for this caller - i.e. left to unlock
  // by progressing. 0 for admins and for a user who has unlocked everything. The
  // sidebar's "Encrypted" teaser uses this to hide once nothing is left locked.
  lockedRemaining: number;
};

export async function getModulesWithMeta(): Promise<ModulesResult> {
  // Was unauthenticated when the catalog was public. It's now entitlement-
  // filtered per caller (admins see everything, a user sees the free first lab
  // plus progression unlocks and grants), so it needs the cookie like every
  // other call. The locked-remaining count rides along in a header so the array
  // body stays unchanged for callers that only want the list.
  const res = await fetch("/api/v1/modules", { credentials: "include" });
  if (!res.ok) throw new Error(`failed to load modules: ${res.status}`);
  const modules: Module[] = await res.json();
  const lockedRemaining = Math.max(0, Number(res.headers.get("X-Modules-Locked") ?? 0) || 0);
  return { modules, lockedRemaining };
}

export async function getModules(): Promise<Module[]> {
  return (await getModulesWithMeta()).modules;
}

// Thrown by createSession on failure. For a "you already have a lab running"
// refusal (reason "user_session_limit") the backend also names the offending
// session, so the caller can offer to stop it and start this one - see LabPage.
export type StartSessionError = Error & {
  reason?: string;
  conflictSessionId?: string;
  conflictModule?: string;
};

export async function createSession(moduleSlug: string, driverIds?: string[]): Promise<Session> {
  let res: Response;
  try {
    res = await fetch("/api/v1/sessions", {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      // drivers is only meaningful for the research labs; the backend ignores it
      // otherwise and re-checks entitlement when it resolves the ids to bytes.
      body: JSON.stringify({ module: moduleSlug, drivers: driverIds }),
    });
  } catch {
    // fetch() only rejects (no Response at all) on a network-level failure -
    // server unreachable, offline, DNS. The browser's message for that is a bare
    // "Failed to fetch", which is meaningless to a student. Replace it with an
    // actionable line and a "network" reason the UI can recognise; every other
    // failure below has a real HTTP response to describe.
    const err = new Error(
      "Couldn’t reach the server. Check your connection and try again.",
    ) as StartSessionError;
    err.reason = "network";
    throw err;
  }
  // Success bodies are a Session; refusals are {error, reason, conflict*}. Read
  // it loosely so we can carry the conflict details onto the thrown error.
  const body = (await res.json().catch(() => null)) as
    | (Session & {
        error?: string;
        reason?: string;
        conflictSessionId?: string;
        conflictModule?: string;
      })
    | null;
  if (!res.ok || !body || body.status === "error") {
    // Prefer the backend's own message (quota, "you already have a lab running",
    // capacity - all actionable). When there isn't one, don't leak "failed to
    // start session: 500": a 5xx with no body is our side failing (or an
    // unreachable backend that the proxy surfaced as 5xx rather than a fetch
    // rejection - the common case in practice), so say that plainly and let the
    // reason drive a "contact us" affordance. Everything else gets a calm generic.
    const serverError = !body?.error && (res.status >= 500 || !body);
    const err = new Error(
      body?.error ||
        (serverError
          ? "Something went wrong starting your lab. Please try again in a moment."
          : "Couldn’t start your lab. Please try again."),
    ) as StartSessionError;
    err.reason = body?.reason || (serverError ? "server_error" : undefined);
    err.conflictSessionId = body?.conflictSessionId;
    err.conflictModule = body?.conflictModule;
    throw err;
  }
  return body;
}

// Look up a session by ID - used to reconnect the lab UI to a still-running
// session after a reload or navigating back (see LabPage.tsx). Requires auth
// and ownership (same as the backend route). A recently-reaped session comes
// back as a normal 200 whose status is "ended" (carrying the reason - see
// Session.reason), so the UI can say why it stopped. Only once the tombstone
// ages out does the lookup 404, which we map to null - "gone", all we can still
// say is that it's no longer there - so the caller clears its stale local state.
export async function getSession(id: string): Promise<Session | null> {
  const res = await fetch(`/api/v1/sessions/${id}`, {
    credentials: "include",
  });
  if (res.status === 404) return null;
  if (!res.ok) throw new Error(`failed to load session: ${res.status}`);
  return res.json();
}

export async function deleteSession(id: string): Promise<void> {
  const res = await fetch(`/api/v1/sessions/${id}`, {
    method: "DELETE",
    credentials: "include",
  });
  if (!res.ok && res.status !== 204) {
    throw new Error(`failed to stop session: ${res.status}`);
  }
}

// Extend a research lab's hard cap by 2h (up to a 48h server-side ceiling),
// returning the session with its freshly-bumped expiresAt so the countdown can
// refresh. Only research labs are extendable; the backend 403s otherwise and
// 409s once at the cap - both surface here as a thrown error the caller can show.
export async function extendSession(id: string): Promise<Session> {
  const res = await fetch(`/api/v1/sessions/${id}/extend`, {
    method: "POST",
    credentials: "include",
  });
  if (!res.ok) {
    // The backend sends a plain-text reason (e.g. "session already at its
    // maximum lifetime"); prefer it over a bare status code.
    const reason = (await res.text().catch(() => "")).trim();
    throw new Error(reason || `failed to extend session: ${res.status}`);
  }
  return res.json();
}

// keepAliveSession is the presence ping: a session's idle timeout reaps it after
// a stretch with no INPUT, and a long fuzzer/build produces only output - so a
// researcher watching it, not typing, would be idle-reaped mid-job. The caller
// fires this on an interval WHILE the lab tab is visible (see LabPage), which
// resets the idle clock; a hidden or closed tab stops pinging, so an abandoned
// session still reaps. Best-effort and fire-and-forget: a failed ping just lets
// the normal idle logic run, so it never throws or blocks the UI.
export async function keepAliveSession(id: string): Promise<void> {
  try {
    await fetch(`/api/v1/sessions/${id}/keepalive`, {
      method: "POST",
      credentials: "include",
    });
  } catch {
    // Network blip - ignore; the session simply follows its normal idle timer.
  }
}

// --- Research driver catalog - see internal/api/drivers.go + admindrivers.go. ---

// Driver is one catalog entry: an uploaded compiled kernel module (.ko) with the
// metadata the backend extracted from the ELF. status is pending | approved |
// denied; visibility is private (uploader only) | catalog (shared). Never
// carries the .ko bytes.
export type Driver = {
  id: string;
  sha256: string;
  filename: string;
  sizeBytes: number;
  modName: string;
  vermagic: string;
  license: string;
  depends: string;
  description: string;
  isSigned: boolean;
  signer?: string;
  status: "pending" | "approved" | "denied";
  visibility: "private" | "catalog";
  uploadedBy: string;
  uploadedAt: string;
  vettedBy?: string;
  vettedAt?: string;
  vetReason?: string;
};

// listDrivers returns the catalog the current research user may pick from: the
// shared vetted catalog plus their own uploads (any status).
export async function listDrivers(): Promise<Driver[]> {
  const res = await fetch("/api/v1/drivers", { credentials: "include" });
  if (!res.ok) throw new Error(await errorText(res, "failed to load drivers"));
  return res.json();
}

// uploadDriver posts the raw .ko bytes (like uploadAvatar), passing the filename
// as a query param since the body is the binary itself. A 409 means the same
// content is already on file - surfaced as an error carrying the server's reason.
export async function uploadDriver(file: File): Promise<Driver> {
  const res = await fetch(`/api/v1/drivers?filename=${encodeURIComponent(file.name)}`, {
    method: "POST",
    credentials: "include",
    body: file,
  });
  if (!res.ok) throw new Error(await errorText(res, "failed to upload driver"));
  return res.json();
}

// adminListDrivers returns the vetting queue for a status (pending by default).
export async function adminListDrivers(status = "pending"): Promise<Driver[]> {
  const res = await fetch(`/api/v1/admin/drivers?status=${encodeURIComponent(status)}`, {
    credentials: "include",
  });
  if (!res.ok) throw new Error(await errorText(res, "failed to load drivers"));
  return res.json();
}

// adminDriverDecision approves (optionally into the shared catalog) or denies an
// uploaded module.
export async function adminDriverDecision(
  id: string,
  decision: "approve" | "deny",
  opts: { catalog?: boolean; reason?: string } = {},
): Promise<void> {
  const res = await fetch(`/api/v1/admin/drivers/${id}/decision`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ decision, catalog: opts.catalog ?? false, reason: opts.reason ?? "" }),
  });
  if (!res.ok) throw new Error(await errorText(res, "failed to record decision"));
}

// driverDownloadUrl is the admin raw-.ko download link (an <a href>, so the
// browser handles the octet-stream attachment).
export function driverDownloadUrl(id: string): string {
  return `/api/v1/admin/drivers/${id}/download`;
}

// --- AI lab-assist grant - see internal/api/ai.go. ---

// AiGrant is the paste-ready config a student hands to an external AI: a scoped,
// short-lived key plus the API it may call. The key is confined to this one
// session's /ai routes and can never submit flags or stop the lab.
export interface AiGrant {
  sessionId: string;
  token: string;
  expiresAt: string;
  ttlSeconds: number;
  baseUrl: string;
  miniPrompt: string;
  api: Record<string, string>;
}

// Mint (or regenerate) an AI grant for a running session the caller owns.
// Regenerating invalidates any previous key immediately (the server keeps only
// the newest).
export async function createAiGrant(sessionId: string): Promise<AiGrant> {
  const res = await fetch(`/api/v1/sessions/${sessionId}/ai-grant`, {
    method: "POST",
    credentials: "include",
  });
  if (!res.ok) {
    throw new Error(`failed to create AI grant: ${res.status}`);
  }
  return res.json();
}

// Revoke the session's AI grant (the student closed the panel). Best-effort: a
// failure here still leaves the 15-minute expiry and lab-close as backstops.
export async function revokeAiGrant(sessionId: string): Promise<void> {
  const res = await fetch(`/api/v1/sessions/${sessionId}/ai-grant`, {
    method: "DELETE",
    credentials: "include",
  });
  if (!res.ok && res.status !== 204) {
    throw new Error(`failed to revoke AI grant: ${res.status}`);
  }
}

// Report which lab tab the student is viewing, so /ai/context can tell the AI
// where their attention is. Best-effort: a failure just means a slightly staler
// hint for the AI, never a broken UI.
export async function setActiveTab(sessionId: string, tab: string): Promise<void> {
  await fetch(`/api/v1/sessions/${sessionId}/active-tab`, {
    method: "PUT",
    credentials: "include",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ tab }),
  }).catch(() => {});
}

// Report which guided-course step the student is on, so /ai/context can tell the
// AI where they are in the lesson and the tutor can pace with them rather than
// jumping to the capture. Best-effort, same as setActiveTab.
export async function setGuideStep(sessionId: string, index: number, total: number): Promise<void> {
  await fetch(`/api/v1/sessions/${sessionId}/guide-step`, {
    method: "PUT",
    credentials: "include",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ index, total }),
  }).catch(() => {});
}

// --- Auth - see internal/api/auth.go. ---

// The auth probe the whole app shell gates on (AuthContext). Its two failure
// modes are deliberately distinct so the shell can tell them apart:
//   - 401 -> null: reachable and definitively logged out (draw "Sign in").
//   - timeout / network error / non-401 !ok -> throw: the server is unreachable
//     or unhealthy, which AuthContext retries and, once exhausted, surfaces as
//     the "server issues" state - NOT as a silent logout.
// The timeout bounds a hung backend so a stalled probe can't wedge the shell on
// its loading state forever; an aborted fetch rejects, i.e. counts as unreachable.
export async function getMe(): Promise<User | null> {
  const res = await fetch("/api/v1/auth/me", {
    credentials: "include",
    signal: AbortSignal.timeout(6000),
  });
  if (res.status === 401) return null;
  if (!res.ok) throw new Error(`failed to load current user: ${res.status}`);
  return res.json();
}

export async function logout(): Promise<void> {
  await fetch("/api/v1/auth/logout", {
    method: "POST",
    credentials: "include",
  });
}

export type OAuthCallbackResult = { user: User; token: string };

export async function googleCallback(code: string, redirectUri: string): Promise<User> {
  const res = await fetch("/api/v1/auth/google/callback", {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ code, redirect_uri: redirectUri }),
  });
  if (!res.ok) throw new Error("Google sign-in failed. Please try again.");
  const data = (await res.json()) as OAuthCallbackResult;
  return data.user;
}

export async function githubCallback(code: string, redirectUri: string): Promise<User> {
  const res = await fetch("/api/v1/auth/github/callback", {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ code, redirect_uri: redirectUri }),
  });
  if (!res.ok) throw new Error("GitHub sign-in failed. Please try again.");
  const data = (await res.json()) as OAuthCallbackResult;
  return data.user;
}

// Dev-only username/password login (localhost). The matching backend route is
// mounted only when DEV_LOGIN=1 (never in prod), and this is called only from
// the import.meta.env.DEV-gated form in Login.tsx - so it's dead code in a prod
// build. Any username works; the shared password is the backend's
// DEV_LOGIN_PASSWORD (default "labs"). Returns the signed-in user, same shape as
// the OAuth callbacks.
export async function devLogin(username: string, password: string): Promise<User> {
  const res = await fetch("/api/v1/auth/dev/login", {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password }),
  });
  if (!res.ok) throw new Error("Login failed - check the dev username/password.");
  const data = (await res.json()) as OAuthCallbackResult;
  return data.user;
}

// terminalWsUrl is the websocket the terminal attaches to - an exec into the
// entry pod inside this session's nested cluster (see NestedTerminal and
// internal/labs/terminal.go). ws/wss off the current origin; the browser can't
// attach an Authorization header to a WebSocket, so this relies on the
// same-origin auth cookie being sent automatically (the backend's requireAuth
// accepts it, see internal/api/auth.go's bearerOrCookieToken).
//
// There is no plain-HTTP terminalUrl any more: nothing serves ttyd assets since
// the legacy pod shape was removed (lab-session-rewrite.md, F1).
export function terminalWsUrl(id: string): string {
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${window.location.host}/api/v1/sessions/${id}/terminal/`;
}

// defenderWsUrl is the read-only websocket the Defender pane attaches to - the
// in-guest eBPF sensor's live event feed (eBPF modules only, see
// internal/labs/defender.go). Same-origin-cookie auth as the terminal (a browser
// WebSocket can't carry an Authorization header).
export function defenderWsUrl(id: string): string {
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${window.location.host}/api/v1/sessions/${id}/defender`;
}

// requireAdmin-gated - see internal/api/routes.go / internal/health.
export async function getAdminHealth(): Promise<AdminNamespace[]> {
  const res = await fetch("/api/v1/admin/health", {
    credentials: "include",
  });
  if (!res.ok) throw new Error(`failed to load cluster health: ${res.status}`);
  return res.json();
}

// requireAdmin-gated - see internal/api/routes.go / internal/topology.
export async function getAdminTopology(): Promise<AdminTopology> {
  const res = await fetch("/api/v1/admin/topology", {
    credentials: "include",
  });
  if (!res.ok) throw new Error(`failed to load topology: ${res.status}`);
  return res.json();
}

// --- Profile + API keys (your own account) - see internal/api/profile.go. ---

export type Profile = {
  website: string;
  bio: string;
  x: string;
  linkedin: string;
  hasAvatar: boolean;
};

export type ApiKey = {
  id: string;
  name: string;
  prefix: string;
  // Sensitive capabilities this key is explicitly authorized for (e.g.
  // "drivers:write"). Empty for a general key. Always present ([] not undefined).
  scopes: string[];
  createdAt: string;
  expiresAt?: string;
  lastUsedAt?: string;
};

// Scope a key needs to upload research drivers via the API. Mintable only by a
// user with research-lab access (the backend enforces both). See DriverPicker /
// the drivers upload route.
export const SCOPE_DRIVERS_WRITE = "drivers:write";

export async function getProfile(): Promise<Profile> {
  const res = await fetch("/api/v1/profile", { credentials: "include" });
  if (!res.ok) throw new Error(`failed to load profile: ${res.status}`);
  return res.json();
}

export async function updateProfile(
  website: string,
  bio: string,
  x: string,
  linkedin: string,
): Promise<Profile> {
  const res = await fetch("/api/v1/profile", {
    method: "PATCH",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ website, bio, x, linkedin }),
  });
  if (!res.ok) throw new Error(await errorText(res, "failed to save profile"));
  return res.json();
}

// The display name is the username handle (shown on the leaderboard, and the
// segment in the public /operators/{username} URL). It's not part of the
// autosaved Profile - it's on the auth user - so it has its own explicit-save
// call and its own route. Returns the stored value; callers refreshUser() after
// so the nav and auth context pick up the new handle.
export async function updateUsername(username: string): Promise<{ username: string }> {
  const res = await fetch("/api/v1/profile/username", {
    method: "PATCH",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username }),
  });
  if (!res.ok) throw new Error(await errorText(res, "failed to save display name"));
  return res.json();
}

// Raw bytes, not multipart: the backend detects the image type from the file's
// own magic bytes rather than any declared Content-Type, so there's nothing for
// a form envelope to carry.
export async function uploadAvatar(file: File): Promise<void> {
  const res = await fetch("/api/v1/profile/avatar", {
    method: "POST",
    credentials: "include",
    body: file,
  });
  if (!res.ok) throw new Error(await errorText(res, "failed to upload avatar"));
}

export async function deleteAvatar(): Promise<void> {
  const res = await fetch("/api/v1/profile/avatar", {
    method: "DELETE",
    credentials: "include",
  });
  if (!res.ok) throw new Error(await errorText(res, "failed to remove avatar"));
}

// Cache-busted by the caller when an upload succeeds - the browser would
// otherwise keep showing the old image for the endpoint's max-age.
export function avatarUrl(userId: string, version?: number): string {
  return `/api/v1/users/${userId}/avatar${version ? `?v=${version}` : ""}`;
}

export async function getApiKeys(): Promise<ApiKey[]> {
  const res = await fetch("/api/v1/apikeys", { credentials: "include" });
  if (!res.ok) throw new Error(`failed to load api keys: ${res.status}`);
  return res.json();
}

// `key` in the response is the only time the plaintext exists outside the
// caller's own storage - it is hashed server-side and unrecoverable after this.
export async function createApiKey(
  name: string,
  expiresInDays: number,
  scopes: string[] = [],
): Promise<{ key: string; apiKey: ApiKey }> {
  const res = await fetch("/api/v1/apikeys", {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name, expiresInDays, scopes }),
  });
  if (!res.ok) throw new Error(await errorText(res, "failed to create api key"));
  return res.json();
}

export async function deleteApiKey(id: string): Promise<void> {
  const res = await fetch(`/api/v1/apikeys/${id}`, {
    method: "DELETE",
    credentials: "include",
  });
  if (!res.ok) throw new Error(await errorText(res, "failed to revoke api key"));
}

// --- User management (requireAdmin) - see internal/api/adminusers.go.
//
// Registration is open: anyone can sign up with Google or GitHub. Entitlement
// is not - a new account starts with zero grants and can see and start nothing
// until an admin grants a module here. ---

// The admin users response carries the accounts plus the global daily quota, so
// the usage column can draw each user's 24h consumption against their cap
// (dailyQuotaGibMinutes === 0 means the quota is disabled).
export type AdminUsersResponse = { users: AdminUser[]; dailyQuotaGibMinutes: number };

export async function getAdminUsers(): Promise<AdminUsersResponse> {
  const res = await fetch("/api/v1/admin/users", { credentials: "include" });
  if (!res.ok) throw new Error(`failed to load users: ${res.status}`);
  return res.json();
}

export async function grantLab(userId: string, moduleSlug: string): Promise<void> {
  const res = await fetch(`/api/v1/admin/users/${userId}/grants`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ module: moduleSlug }),
  });
  if (!res.ok) throw new Error(await errorText(res, "failed to grant access"));
}

// Also tears down any session the user has running for that module - see the
// backend handler for why a revoke that leaves the shell open is cosmetic.
export async function revokeLab(userId: string, moduleSlug: string): Promise<void> {
  const res = await fetch(
    `/api/v1/admin/users/${userId}/grants/${encodeURIComponent(moduleSlug)}`,
    { method: "DELETE", credentials: "include" },
  );
  if (!res.ok) throw new Error(await errorText(res, "failed to revoke access"));
}

// Permanently removes the account, its grants (ON DELETE CASCADE), and its
// running sessions. Irreversible - setUserBlocked is the reversible option.
export async function deleteUser(userId: string): Promise<void> {
  const res = await fetch(`/api/v1/admin/users/${userId}`, {
    method: "DELETE",
    credentials: "include",
  });
  if (!res.ok) throw new Error(await errorText(res, "failed to delete user"));
}

// Blocking denies every login path and kills the user's running sessions.
// Grants are preserved, so unblocking restores exactly the prior access.
export async function setUserBlocked(userId: string, blocked: boolean): Promise<void> {
  const res = await fetch(`/api/v1/admin/users/${userId}/blocked`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ blocked }),
  });
  if (!res.ok) throw new Error(await errorText(res, "failed to update user"));
}

// Set (number) or clear (null) a user's per-user daily quota override, in
// GiB-minutes. null falls back to the global default; 0 exempts the user; a
// positive value is their cap. See accounts.SetUserQuota.
export async function setUserQuota(userId: string, gibMinutes: number | null): Promise<void> {
  const res = await fetch(`/api/v1/admin/users/${userId}/quota`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ gibMinutes }),
  });
  if (!res.ok) throw new Error(await errorText(res, "failed to update quota"));
}

// The admin endpoints answer with a plain-text reason (http.Error), which is
// more useful to surface than a bare status code - "cannot block your own
// account" is the one an admin is most likely to hit.
async function errorText(res: Response, fallback: string): Promise<string> {
  const body = (await res.text().catch(() => "")).trim();
  return body || `${fallback}: ${res.status}`;
}

// --- Admin -> Lab Management. Changing a lab's duration changes how long a
// student's work survives, so these are admin-gated server-side and validated
// rather than clamped (the API answers 400 with the actual reason). Sessions
// already running keep the timing they started with. ---

export async function getLabSettings(): Promise<LabSetting[]> {
  const res = await fetch("/api/v1/admin/labs/settings", { credentials: "include" });
  if (!res.ok) throw new Error(`failed to load lab settings: ${res.status}`);
  return res.json();
}

export async function setLabSetting(
  slug: string,
  durationMinutes: number,
  idleMinutes: number,
): Promise<void> {
  const res = await fetch(`/api/v1/admin/labs/settings/${slug}`, {
    method: "PUT",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ durationMinutes, idleMinutes }),
  });
  if (!res.ok) throw new Error(await errorText(res, "failed to save lab settings"));
}

// Drops the override so the lab falls back to the built-in default.
export async function resetLabSetting(slug: string): Promise<void> {
  const res = await fetch(`/api/v1/admin/labs/settings/${slug}`, {
    method: "DELETE",
    credentials: "include",
  });
  if (!res.ok) throw new Error(await errorText(res, "failed to reset lab settings"));
}

// --- A lab's guided course + the caller's progress in it.
//
// Course content is embedded in the backend binary and served pre-rendered:
// `body`, `aha` and `explain` arrive as HTML produced by the backend's own
// restricted Markdown renderer (internal/course/prose.go), which escapes before
// it introduces any markup and cannot emit raw HTML, SVG or links. Content is
// also PR-reviewed rather than user-submitted. Both of those are why the player
// can set this HTML directly.
//
// Progress is keyed by (user, module) server-side, never by session, so it
// survives a session being reaped and the lab being restarted another day. ---

export type CourseStep = {
  slug: string;
  index: number;
  kind: "concept" | "quiz" | "flag";
  ey: string;
  eyt: string;
  tag?: string;
  chip?: { label: string; tone: "accent" | "warn" | "danger" };
  diagram?: unknown;
  title?: string;
  body?: string;
  aha?: string;
  revealLabel?: string;
  q?: string;
  opts?: { t: string; correct?: boolean }[];
  explain?: string;
};

export type Course = { moduleSlug: string; steps: CourseStep[] | null };

export type CourseProgress = {
  moduleSlug: string;
  answers: Record<string, number>;
  // complete is the captured CTF flag, not the quizzes. It flips true only once
  // the student submits the correct flag (submitFlag); the quizzes are formative
  // and never move it. answered/quizCount track self-check progress only.
  complete: boolean;
  answered: number;
  quizCount: number;
};

export type FlagResult = { correct: boolean; complete: boolean };

export async function getCourse(slug: string): Promise<Course> {
  const res = await fetch(`/api/v1/modules/${slug}/course`, { credentials: "include" });
  if (!res.ok) throw new Error(`failed to load course: ${res.status}`);
  return res.json();
}

export async function getCourseProgress(slug: string): Promise<CourseProgress> {
  const res = await fetch(`/api/v1/modules/${slug}/progress`, { credentials: "include" });
  if (!res.ok) throw new Error(`failed to load progress: ${res.status}`);
  return res.json();
}

// Records one answer and returns progress. Quizzes are formative: this stores
// the answer (right or wrong) but never marks the lab complete - only the flag
// does that (submitFlag). The client never asserts completion.
export async function putCourseAnswer(
  slug: string,
  stepSlug: string,
  option: number,
): Promise<CourseProgress> {
  const res = await fetch(`/api/v1/modules/${slug}/progress`, {
    method: "PUT",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ stepSlug, option }),
  });
  if (!res.ok) throw new Error(await errorText(res, "failed to save your answer"));
  return res.json();
}

// Submits the CTF flag the student captured. The server checks it against the
// module's expected flag (held server-side, never served) and, on a match, marks
// the lab solved. A wrong flag is a normal result with correct=false, not an
// error - so only a transport/auth failure throws.
export async function submitFlag(slug: string, flag: string): Promise<FlagResult> {
  const res = await fetch(`/api/v1/modules/${slug}/flag`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ flag }),
  });
  if (!res.ok) throw new Error(await errorText(res, "failed to submit the flag"));
  return res.json();
}

// Clears the caller's progress in a lab - answers, completion and solve time -
// so they can restart it and retake the self-checks. Idempotent server-side.
export async function resetCourseProgress(slug: string): Promise<void> {
  const res = await fetch(`/api/v1/modules/${slug}/progress`, {
    method: "DELETE",
    credentials: "include",
  });
  if (!res.ok) throw new Error(await errorText(res, "failed to restart the lab"));
}

// --- Leaderboard.
//
// Public: the top ranks render on the landing page for signed-out visitors, so
// this call works without a session. The server reveals only the leaders until
// the caller has cleared a lab, and never discloses the total operator count -
// so `entries` is the top few (with `locked: true`) until you're on the board
// yourself, at which point it's the full list. Stats are all real: labsCompleted
// and fastestSolveSeconds come from completion records, longestStreakDays from
// session activity. fastestSolveSeconds is null for an operator with no solve
// recorded yet. `distinction` is a category slug the holder of each superlative
// carries; the UI pairs it with that row's own number. ---

export type LeaderboardEntry = {
  rank: number;
  // Only present for a signed-in viewer - the public payload omits internal
  // account ids. Avatars only render when signed in, so this is enough.
  userId?: string;
  username: string;
  hasAvatar: boolean;
  x?: string;
  linkedin?: string;
  labsCompleted: number;
  fastestSolveSeconds: number | null;
  longestStreakDays: number;
  // Cumulative labs-cleared sampled over a window shared by the whole board, for
  // the trend sparkline next to the labs total. All real (derived from the same
  // completion records as labsCompleted); null for an operator with no clears.
  labsTrend: number[] | null;
  distinction?: "fastest escape" | "longest streak" | "most labs cleared";
  isViewer?: boolean;
};

export type Leaderboard = {
  entries: LeaderboardEntry[];
  revealed: number;
  locked: boolean;
  viewer: {
    authenticated: boolean;
    hasClearedLab: boolean;
    rank?: number | null;
    // The viewer's own labs count, sent so the climb widget can frame the gap
    // even when the viewer's row is gated out of `entries` (a locked board only
    // reveals the top ranks).
    labsCompleted: number;
  };
  // The operator ranked immediately ahead of the viewer on the full board - the
  // nearest one to overtake - even when the board is locked. Absent when the
  // viewer leads or there's no signed-in viewer.
  nextTarget?: LeaderboardEntry;
};

export async function getLeaderboard(): Promise<Leaderboard> {
  const res = await fetch("/api/v1/leaderboard", { credentials: "include" });
  if (!res.ok) throw new Error(`failed to load leaderboard: ${res.status}`);
  return res.json();
}

// --- Public operator profiles.
//
// The everyone-can-see view of an operator, behind /u/:username and the landing
// showcase. Same disclosure as the leaderboard - handle, real stats, and the
// self-published bio/website - plus the daily activity calendar, which is real
// finished-session counts (the same signal as the streak), NOT a generated
// shape: an idle stretch shows as empty. `activity` lists only active days
// (oldest first); the heatmap fills the gaps across `activityDays`. Avatars have
// their own public URL (publicAvatarUrl) since the profile is viewable signed
// out. ---

export type DayActivity = { date: string; count: number };

export type PublicProfile = {
  username: string;
  bio: string;
  website: string;
  x?: string;
  linkedin?: string;
  hasAvatar: boolean;
  joinedAt: string;
  rank: number;
  totalOperators: number;
  labsCompleted: number;
  fastestSolveSeconds: number | null;
  longestStreakDays: number;
  activity: DayActivity[];
  activityDays: number;
};

// Thrown by getPublicProfile for an unknown handle so the page can render a
// "no such operator" state instead of a generic error.
export class ProfileNotFound extends Error {}

export async function getPublicProfile(username: string): Promise<PublicProfile> {
  const res = await fetch(`/api/v1/operators/${encodeURIComponent(username)}`);
  if (res.status === 404) throw new ProfileNotFound(username);
  if (!res.ok) throw new Error(`failed to load profile: ${res.status}`);
  return res.json();
}

export async function getShowcaseOperators(): Promise<PublicProfile[]> {
  const res = await fetch("/api/v1/operators");
  if (!res.ok) throw new Error(`failed to load operators: ${res.status}`);
  return res.json();
}

// Public avatar, keyed by handle so no internal id rides a public page.
export function publicAvatarUrl(username: string): string {
  return `/api/v1/operators/${encodeURIComponent(username)}/avatar`;
}

// --- The student's own lab topology.
//
// A different endpoint from the admin map, backed by a different builder: it
// reads only the caller's nested cluster, so the outer cluster's node names,
// kernel versions, dashboard paths, signals and other users' sessions never
// enter the payload. That is structural rather than a filter - see
// internal/topology/scoped.go. The renderer is shared; the data source is not.
//
// `boundary` is the id of an opaque node standing for everything outside the
// student's lab. It carries no name, version or address of ours. ---

export type ScopedTopology = {
  boundary: string;
  namespaces?: TopoNamespace[];
  edges?: TopoEdge[];
  // Set when the lab cluster isn't reachable yet - still provisioning, or just
  // torn down. A normal state, not a failure.
  error?: string;
};

export async function getSessionTopology(sessionId: string): Promise<ScopedTopology> {
  const res = await fetch(`/api/v1/sessions/${sessionId}/topology`, { credentials: "include" });
  if (!res.ok) throw new Error(`failed to load lab topology: ${res.status}`);
  return res.json();
}
