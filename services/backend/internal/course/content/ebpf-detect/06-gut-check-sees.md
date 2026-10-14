---
kind: quiz
ey: GUT-CHECK
eyt: Did it land?
tag: trick question
q: You run a `SUID` binary and `id` prints nothing alarming. What does the `kprobe` on `commit_creds` report?
opts:
  - t: Nothing - the shell looked normal, so there was nothing to see
  - t: A process being given root credentials - the `euid` 0 the kernel just installed
    correct: true
  - t: The contents of the file you ran
  - t: A login event for the root account
explain: The shell's calm output is userspace theatre. Down in the kernel, the `SUID` exec still called `commit_creds` to install `euid` 0, and the `kprobe` fired at exactly that instant - reporting the credential change and the process behind it. The sensor sees the privilege jump precisely because it is not watching the shell.
---
