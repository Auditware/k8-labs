---
kind: quiz
ey: GUT-CHECK
eyt: Did it land?
tag: trick question
q: You want to catch the exact moment a process becomes root. Where do you look?
opts:
  - t: The shell's command history
  - t: The system auth log
  - t: '`commit_creds` in the kernel, as it installs the credentials'
    correct: true
  - t: The timestamp on the SUID file
explain: History and logs are userspace, and userspace can stay silent or lie - the shell wrote nothing when your `euid` changed, and a log only records what some program chose to record. The SUID file's timestamp tells you when it was created, not when it ran. Only `commit_creds` sees the credential actually being installed, whether or not anyone bothered to log it.
---
