---
kind: quiz
ey: GUT-CHECK
eyt: Before you go
tag: trick question
q: You confirm `backup-tool` is `SUID` root and builds a `cp` command from your argument. What is the move?
opts:
  - t: Pass an argument containing a shell command after a `;`
    correct: true
  - t: Find the root password in the binary
  - t: Change the SUID bit to make it run as you
  - t: Copy the flag with `cp` - the tool already runs as root
explain: The tool shells out your argument, so ending the `cp` with a `;` and running your own command runs it as root - that is the injection. Reading the password from the binary is the wrong mental model; there is no password, the SUID bit is the mechanism. You cannot change the SUID bit without already being root. And the last option is closer than it looks - the tool DOES run as root - but `cp` writes into /backup, it does not hand you the flag's contents; you want a shell, not a copy.
---
