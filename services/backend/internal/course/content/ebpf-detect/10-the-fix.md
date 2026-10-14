---
kind: concept
ey: RECAP
eyt: Why this matters
chip:
  label: TAKEAWAY
  tone: accent
title: You cannot hide the privilege jump from the kernel
aha: An attacker controls the shell, the logs, and their own tools - so a defender who only watches those is watching the attacker's stage. Watching the kernel means watching the one thing the attacker still has to ask permission from.
diagram:
  caption: userspace can lie; the credential install cannot be skipped
  shapes:
    - {type: box, x: 40, y: 66, w: 250, h: 54, label: 'shell + logs', sub: 'attacker can silence these', tone: danger, delay: 0}
    - {type: box, x: 40, y: 170, w: 250, h: 54, label: 'commit_creds in the kernel', sub: 'still has to run', tone: accent, delay: 0.5}
    - {type: seal, cx: 440, cy: 148, r: 30, delay: 0.9}
    - {type: label, x: 440, y: 214, text: 'nowhere for the jump to hide', tone: accent, delay: 1.1}
---
A shell stays quiet, a log can be scrubbed, a tool can be renamed - all of it is
userspace, and an attacker who reaches root owns all of it. But to reach root at
all, the kernel still had to install the credentials, and that still ran through
`commit_creds`.

That is why defenders instrument the kernel with `eBPF` and not just the logs.
Userspace can lie about what happened; the credential install cannot be skipped.
You just watched your own root, from the one place it could not hide.
