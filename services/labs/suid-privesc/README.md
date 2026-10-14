# suid-privesc - SUID privilege escalation (CWE-78)

A local privilege-escalation lab in the nested-cluster shape. The student lands
in an unprivileged shell and must escalate to root by exploiting an
OS-command-injection flaw in a SUID-root binary, then read a root-only flag.

## Shape

Everything is in the module's manifest bundle
(`services/backend/internal/labs/modules/suid-privesc.yaml`), applied into
the session's disposable nested k3s cluster - no per-module image. The
vulnerable binary is a musl-static `backup-tool`, compiled for amd64 and arm64
and carried as base64 in a ConfigMap (~23-35 KB per arch, both well under the
1 MB ConfigMap limit). A root initContainer selects the arch by `uname -m`,
installs it SUID-root into a shared `emptyDir`, and writes a root-only flag.
The student's shell container runs as uid 10001, so the SUID binary is the only
way up.

Winning gets root **inside a container, inside a throwaway nested cluster,
inside a microVM** (Kata in prod, runc locally) - the same containment
`ns-escape` relies on. The flag is bait, not a secret (see `_template`).

## The flaw

`backup-tool` builds `cp <argv[1]> /backup/` and runs it through `/bin/sh`
without sanitizing the argument. A `;` or `$(...)` in the argument runs as root.
The foothold image is Alpine, so `/bin/sh` is BusyBox `ash`, which - unlike
`bash` - does **not** drop the inherited effective-uid when a SUID-root binary
execs it. So the injected command keeps euid 0 and runs as root. (`bash` resets
euid back to the real uid at startup unless launched with `-p`; BusyBox `ash`
simply never does that reset, which is why no `-p` is needed here. The old
`suid-privesc-101` made the same point on Debian/dash.)

## Solution (spoiler)

`backup-tool` runs `cp <argv[1]> /backup/` via `/bin/sh`, so it appends ` /backup/`
after your input. Split with `;` to run your own command, and end it with a `#` so
the trailing `/backup/` is commented out instead of landing as an argument on it
(e.g. bare `x; id` runs `id /backup/`, which just errors). The leading `cp` (now
missing its source) prints a harmless BusyBox usage line first.

```sh
backup-tool "x; id #"                 # -> uid=0(root): the injected command runs as root
backup-tool "x; cat /app/flag.txt #" # read the root-only flag
backup-tool "x; sh #"                 # or drop into an interactive root shell
# k8labs{suid_command_injection_to_root_via_backup_tool}
```

## The fix

Copy the file directly (open source, write destination) with no shell in the
loop, so there is no command line for a `;` to hide in. Passing user input to
`/bin/sh` was the whole bug.

## What a defender sees

Under runc (local) the host kernel observes the escalation, so Tetragon's
`commit_creds` hook fires on the euid transition. Under Kata (prod) the microVM
kernel boundary hides it from host eBPF - see
`apps/tetragon/templates/process-creds-changed.yaml`. In-cluster telemetry (a
sensor inside the nested cluster) is the way to observe lab activity under Kata;
that is a separate, planned piece of work.
