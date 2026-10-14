---
kind: quiz
ey: GUT-CHECK
eyt: Did it land?
tag: trick question
q: You inject `sh` through `backup-tool` and it opens a shell. What user is that shell?
opts:
  - t: Root - `backup-tool` runs as root, so the shell it starts does too
    correct: true
  - t: Your own user - shells always start as the person who typed the command
  - t: Root, but only for the very first command you type
  - t: Nobody can tell until you run `whoami`
explain: The shell inherits the identity of the process that started it, and `backup-tool` is running as root - so the shell is a root shell, for every command, not just the first. It is not tied to who "typed" it; a process's user comes from how it was started, not from the keyboard. (The one real-world caveat - some SUID programs only borrow root's *effective* id and a freshly started shell can hand it back - but a program like this one, which takes on the root identity fully, leaves nothing to hand back.)
---
