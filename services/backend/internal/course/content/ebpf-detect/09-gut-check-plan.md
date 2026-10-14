---
kind: quiz
ey: GUT-CHECK
eyt: Before you switch tabs
tag: trick question
q: You escalate with `backup-tool` and open the `Defender` pane. What are you looking for?
opts:
  - t: Your command, echoed back in the feed
  - t: A `commit_creds` event installing `euid` 0, caused by your escalation
    correct: true
  - t: The contents of the flag file
  - t: An error, because eBPF blocks the escalation
explain: The `Defender` feed is `Tetragon`'s stream from its `kprobe` on `commit_creds`. The instant your SUID exec installs root credentials, that hook fires and the feed highlights the transition to `euid` 0. The sensor does not echo your typing or read your files, and it does not block anything here - it observes. What you are hunting for is your own privilege jump, seen from the kernel.
---
