import { describe, it, expect } from "vitest";
import { parse } from "./DefenderPane";

// Fixtures are the real event shapes captured from the in-guest Tetragon sensor
// during the nested-flow validation (docs/architecture/ebpf-per-session.md).
describe("DefenderPane parse", () => {
  it("reads a process_exec as an exec row with binary + args", () => {
    const line = JSON.stringify({
      process_exec: { process: { binary: "/usr/local/bin/backup-tool", arguments: '"x; id"', pid: 92035, uid: 10001 } },
      time: "2026-07-23T09:04:53.3Z",
    });
    const r = parse(line, 1)!;
    expect(r.kind).toBe("exec");
    expect(r.text).toContain("/usr/local/bin/backup-tool");
    expect(r.text).toContain('"x; id"');
  });

  it("flags euid 0 with a non-root real uid as the escalation (the divergence)", () => {
    // The real captured escalation shape: effective root borrowed onto the
    // still-unprivileged user (uid 10001). Binary is unresolved (fires
    // mid-execve), so the row leads with the credential fact, not the name.
    const line = JSON.stringify({
      process_kprobe: {
        function_name: "commit_creds",
        process: { pid: 93643, flags: "unknown" },
        args: [{ process_credentials_arg: { uid: 10001, gid: 10001, euid: 0, egid: 10001, suid: 0, fsuid: 0 } }],
      },
      time: "2026-07-23T09:04:53.4Z",
    });
    const r = parse(line, 2)!;
    expect(r.kind).toBe("escalation");
    // The line carries the full uid triple, so the divergence (real uid still
    // 10001, effective uid 0) is legible on the line itself - colour only draws
    // the eye. No detail from the event is dropped.
    expect(r.text).toContain("uid 10001");
    expect(r.text).toContain("euid 0");
    expect(r.text).toContain("suid 0");
    expect(r.text).toContain("root credentials installed");
  });

  it("reveals the flag (the event message) on the escalation row", () => {
    // ebpf-detect rides the flag on Tetragon's `message`. It must surface on the
    // student's own escalation - that is the reward for reading the detection.
    const line = JSON.stringify({
      process_kprobe: {
        function_name: "commit_creds",
        // Tetragon nests the policy message under process_kprobe (verified live).
        message: "k8labs{ebpf_caught_the_commit_creds_transition}",
        process: { pid: 93643 },
        args: [{ process_credentials_arg: { uid: 10001, euid: 0, suid: 0 } }],
      },
    });
    const r = parse(line, 20)!;
    expect(r.kind).toBe("escalation");
    expect(r.text).toContain("k8labs{ebpf_caught_the_commit_creds_transition}");
  });

  it("does NOT leak the message flag on system root churn (uid == euid == 0)", () => {
    // The same message rides every commit_creds event, including the system's own
    // root churn. It must stay hidden there so the flag only appears once the
    // student escalates - never before.
    const line = JSON.stringify({
      process_kprobe: {
        function_name: "commit_creds",
        message: "k8labs{ebpf_caught_the_commit_creds_transition}",
        process: { binary: "/bin/runc" },
        args: [{ process_credentials_arg: { uid: 0, euid: 0, suid: 0 } }],
      },
    });
    const r = parse(line, 21)!;
    expect(r.kind).toBe("creds-root");
    expect(r.text).not.toContain("k8labs{");
  });

  it("demotes euid 0 with real uid 0 to system root churn, not the escalation", () => {
    // runc/k3s/containerd install root creds constantly, but they are already
    // root (uid == euid == 0) - this is the flood the escalation must stand out
    // from, so it must NOT be the loud escalation kind.
    const line = JSON.stringify({
      process_kprobe: {
        function_name: "commit_creds",
        process: { binary: "/bin/runc" },
        args: [{ process_credentials_arg: { uid: 0, gid: 0, euid: 0, egid: 0, suid: 0, fsuid: 0 } }],
      },
    });
    const r = parse(line, 3)!;
    expect(r.kind).toBe("creds-root");
    expect(r.kind).not.toBe("escalation");
    expect(r.text).toContain("uid 0, euid 0");
    expect(r.text).toContain("/bin/runc");
  });

  it("treats a non-root commit_creds as a plain creds row, not an alert", () => {
    const line = JSON.stringify({
      process_kprobe: {
        function_name: "commit_creds",
        process: { binary: "/bin/sh" },
        args: [{ process_credentials_arg: { uid: 100, euid: 100, suid: 100 } }],
      },
    });
    const r = parse(line, 4)!;
    expect(r.kind).toBe("creds");
    expect(r.text).toContain("uid 100, euid 100, suid 100");
  });

  it("falls back to a plain root-cred row when the real uid is absent (can't judge divergence)", () => {
    const line = JSON.stringify({
      process_kprobe: {
        function_name: "commit_creds",
        process: { binary: "/bin/k3s" },
        args: [{ process_credentials_arg: { euid: 0 } }],
      },
    });
    const r = parse(line, 5)!;
    expect(r.kind).toBe("creds-root");
    expect(r.text).toContain("/bin/k3s");
  });

  it("ignores kprobe events that are not commit_creds", () => {
    const line = JSON.stringify({ process_kprobe: { function_name: "something_else", process: { binary: "/x" } } });
    expect(parse(line, 6)).toBeNull();
  });

  it("surfaces a non-JSON line as a raw row rather than dropping it", () => {
    const r = parse("not json at all", 7)!;
    expect(r.kind).toBe("raw");
  });
});
