# Research driver catalog - upload, vet, pick, load

Status: **Draft / proposed** (2026-09-03). Not built. Sibling of
`kasan-research-lab.md`; extends the `driver-research-{dev,hard}` labs so a
research user can bring their own compiled kernel modules instead of only the
planted sample, an admin vets what enters a shared catalog, and the UI lets a
user pick which vetted drivers to install and load into a session.

This doc exists because the feature was discussed but never written down. It is
the spec, not a record of anything shipped.

## Goal

Turn the research workstation from "one baked-in sample driver" into "a curated
catalog of compiled `.ko`s a user can choose from per session":

1. A research-grant user **uploads a compiled `.ko`**. The backend parses it,
   extracts `modinfo` (name, vermagic, license, depends, description), computes a
   content hash, and files it as `pending`.
2. An **admin vets** it: sees the metadata, the hash, the vermagic/kernel
   compatibility, the uploader, and a signature/provenance view, then
   allow/denies it into the shared catalog with a reason.
3. In the lab UI, a user **picks from the vetted catalog** (plus their own
   approved uploads) which drivers to install for a session.
4. At provision, the selected `.ko`s are **injected into the workstation** and
   loadable in-session (a helper lists them and `insmod`s on request; auto-load
   optional).

## Non-goals

- **Not** rebuilding or replacing the guest kernel - the module build tree is
  `modules_prepare`-only and the kernel is prod-only (see `kasan-research-lab.md`).
- **Not** arbitrary binary execution beyond loadable modules. The catalog is
  `.ko`s, not a general file store.
- **Not** lifting the default-deny NetworkPolicy. Drivers arrive through the
  backend upload path, never an in-guest `git clone`/`apt`.
- **Not** per-user kernel isolation changes. The isolation wall stays
  Firecracker/jailer; this feature does not touch it.

## Threat model and what is actually new

The load itself is already inside the existing threat model. A research user is
**already root** in a disposable microVM whose **kernel is the intended target**,
and can already author module source in-session and `insmod` it. Loading a
*precompiled* `.ko` grants nothing a from-source build did not. The escape wall
is the VMM (the escape-bounty wall), not module loading, and it is unchanged.

So admin vetting here is **not** an isolation control. The genuinely new surface
is narrower and worth scoping tightly:

- **The ingestion pipeline.** Today the only path into the airgapped guest is the
  terminal. Upload adds a file path into the platform. Validate it is a real ELF
  relocatable (`ET_REL`) `.ko`, cap size, rate-limit, never trust the filename,
  store by content hash, and never execute or `insmod` it **on the host or in the
  backend** - it is only ever loaded inside a guest microVM.
- **A shared catalog.** A driver an admin promotes to global is visible to every
  research user, so vetting is really abuse/curation control: no malware hosting,
  no exfil tooling (egress is denied anyway), a trustworthy shared library, and
  dedup by hash.
- **Kernel module signatures are NOT load-enforced.** The research kernels build
  with `MODVERSIONS` off and no `CONFIG_MODULE_SIG` enforcement (that is why
  `insmod` works today). "Signature" in the vetting UI therefore means
  **provenance + integrity**, not a load-time gate: the SHA-256 the platform
  computes, the `modinfo` `signer`/`sig_id` fields **if** the uploader signed it,
  and an optional platform-applied detached signature for audit. Vetting is a
  human allow/deny, not a cryptographic boot check.
- **Vermagic is the real compatibility gate.** A `.ko` loads only if its vermagic
  matches the guest kernel (version + SMP + preempt + `mod_unload`). dev and hard
  share a version but not their config, so surface per-kernel compatibility up
  front rather than letting `insmod` fail opaquely mid-session.

## Data model

New table (Postgres; no object store exists in the stack - `.ko`s are small,
~10 KB to low single-digit MB, so `bytea` is the pragmatic store; revisit an
object store only if a size or volume ceiling is hit).

```
research_drivers
  id            uuid pk
  sha256        text unique         -- content address; dedup + integrity
  filename      text                -- original name, display only, never a path
  size_bytes    int
  content       bytea               -- the .ko itself
  -- extracted at upload from modinfo/ELF, never trusted from the client:
  mod_name      text
  vermagic      text
  license       text
  depends       text
  description   text
  is_signed     bool
  signer        text null           -- modinfo signer, if present
  -- lifecycle:
  status        text                -- pending | approved | denied
  visibility    text                -- private (uploader only) | catalog (global)
  uploaded_by   text  fk users(id)
  uploaded_at   timestamptz
  vetted_by     text null fk users(id)
  vetted_at     timestamptz null
  vet_reason    text null
  compatible    text[]              -- module slugs whose kernel vermagic matches
```

An `approved` + `visibility=catalog` driver is in the shared catalog. An
`approved` + `visibility=private` driver is usable only by its uploader. A
`pending` driver is usable by no one until an admin acts.

## Upload and parse

`POST /api/v1/research/drivers` (multipart, one `.ko`), gated on holding a
research-lab grant (reuse the entitlement check that gates the labs themselves).

On receipt the backend:
1. Enforces size cap and content type; reads the bytes.
2. **Validates it is an ELF `ET_REL` with a `.modinfo` section** - reject
   anything else outright (this is the load-bearing input check).
3. Extracts `modinfo` fields and computes `sha256`. Parsing is pure Go over the
   ELF (`debug/elf` + a `.modinfo` NUL-split), so **nothing runs the file**.
4. Computes `compatible[]` by matching the `.ko` vermagic against each research
   module's known guest vermagic.
5. Dedups on `sha256`; inserts as `pending`, `visibility=private`.

Store nothing on the host filesystem and never `insmod` during ingestion.

## Vetting (admin)

Slots into the existing `requireAdmin` surface (`adminusers.go` / `adminlabs.go`
neighbours):

- `GET  /api/v1/admin/research/drivers?status=pending` - list with the full
  extracted metadata, hash, uploader, size, `is_signed`/`signer`, and
  `compatible[]`. This is the "see the driver's sigs/info to allow/deny" view.
- `GET  /api/v1/admin/research/drivers/{id}/download` - fetch the raw `.ko` for
  out-of-band inspection (an admin's own analysis box), audit-logged.
- `POST /api/v1/admin/research/drivers/{id}/decision` - `{approve|deny, reason,
  visibility}`. Approve can promote to `catalog` or keep `private`.

Admin actions are audit-logged (who/when/reason), same expectation as
grant/block.

## Catalog and picker (user)

- `GET  /api/v1/research/drivers` - the caller's usable set: global `catalog`
  drivers plus their own `approved` `private` ones, each tagged with
  `compatible[]` so the UI can grey out ones that will not load on the module the
  user is about to start.
- The lab UI (the research orientation panel, `ResearchGuide.tsx`, where the
  guided course would be) grows a **"Drivers" picker**: a checklist of usable,
  compatible drivers, plus an **Upload** affordance that hits the upload endpoint
  and shows the new upload sitting in `pending` until an admin clears it.
- Selection is per session. Persist the chosen ids on the session (or pass them
  at create) so provision knows what to inject.

The picker is screen-adaptive like the rest of the lab shell (the research pane
already reflows; the checklist is a plain responsive list).

## Injecting into the session

The backend already applies per-session resources into the nested cluster via
`applyNestedManifests` (see `internal/labs/nested.go`) and gates the research
toolchain import on `SESSION_RESEARCH=1` (`runner.go` / `entrypoint.sh`). Reuse
that seam:

1. At `provisionNested`, after the workstation manifest, the backend creates a
   **nested `Secret`** holding the selected `.ko` bytes keyed by
   `mod_name` + short hash, plus a small `index.json` (name -> vermagic ->
   filename).
2. The `Secret` mounts **read-only** into the workstation at `/opt/drivers/`
   (extend `driver-research.yaml`'s workstation pod with the volume + mount when
   drivers are selected; absent otherwise, so a no-driver session is unchanged).
3. Do **not** auto-`insmod`. A user driver is untrusted-by-the-platform code the
   researcher chose; load is an explicit in-session act, matching how the sample
   driver is `insmod`ed today.

Alternative considered and rejected for v1: a backend-served in-guest fetch
endpoint. It reopens an ingress into the airgapped guest and needs its own auth;
the `Secret` mount reuses machinery that already exists and stays pull-free.

## Loading in-session

Ship a small helper in the workstation image (alongside `/app` and `load.sh`):

- `research-driver ls` - reads `/opt/drivers/index.json`, prints name, vermagic,
  and whether vermagic matches `uname -r`.
- `research-driver load <name>` - `insmod /opt/drivers/<file>.ko`, surfacing the
  real `insmod` error (vermagic mismatch, unknown symbol) rather than swallowing
  it. `KBUILD_MODPOST_WARN` is irrelevant here - these are already compiled.

The Defender pane keeps working unchanged: an `insmod` of a user driver shows the
same `process_exec` / `commit_creds` stream as the sample.

## Limits, quotas, abuse

- Per-user upload quota (count + total bytes) and an upload rate limit.
- Global catalog size cap; dedup by hash already bounds duplicates.
- `denied` rows are retained (with reason) so the same bad `.ko` is not
  re-litigated on every re-upload; a re-upload of a `denied` hash short-circuits.
- Uploads and downloads are audit-logged. The raw-`.ko` admin download is the
  most sensitive read; log it explicitly.

## Access model

Uploading and using drivers requires a **research-lab grant** - the same vetted
population that can open the labs at all (`GrantOnly: true`). No new public
surface: every endpoint is behind auth, and the admin ones behind `requireAdmin`.
A non-granted user sees no picker and gets 403 on the upload route.

## Rollout phases

1. **P1 - storage + ingest.** `research_drivers` table, upload endpoint, ELF/
   modinfo parse, hash/dedup, `pending`. No UI beyond a raw upload. Prove the
   parser rejects non-`.ko` input and extracts vermagic correctly.
2. **P2 - vetting.** Admin list/decision/download endpoints + the admin pane
   view. Approve to `private` first (simplest), then `catalog`.
3. **P3 - picker + injection.** User catalog endpoint, the Drivers picker in the
   research panel, per-session selection, the `Secret`-mount injection, and the
   `research-driver` helper. End-to-end: upload -> admin approve -> pick -> load
   in-session -> Defender shows the `insmod`.
4. **P4 - polish.** Quotas/rate limits, `denied`-hash short-circuit, audit-log
   surfacing, per-kernel compatibility greying in the picker.

## Open questions

- **Storage ceiling.** `bytea` is fine at expected sizes; name the count/byte
  threshold at which an object store (minio) becomes worth standing up.
- **Signing.** Do we want the platform to apply its own detached signature at
  approve time for provenance, even though the kernel does not enforce it? Cheap
  audit value; decide in P2.
- **Sharing granularity.** Is `private` vs global `catalog` enough, or do we need
  per-cohort visibility (share within a team but not globally)? Default to the
  two-level model until there is a concrete need.
- **Dev vs hard compatibility.** Confirm whether a single `.ko` vermagic matches
  both research kernels or whether the picker must show them as distinct targets.
