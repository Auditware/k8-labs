---
kind: concept
ey: THE LAB
eyt: Your starting position
tag: go and do it
title: Here is what you have been given
aha: You already know how to get root from earlier work - a SUID binary that hands your input to a shell. Do not linger on the escalation. The new skill is turning to the Defender pane and reading what the kernel saw.
revealLabel: how to start
diagram:
  caption: escalate on the left; find your own footprint on the right
  shapes:
    - {type: box, x: 40, y: 120, w: 165, h: 60, label: 'your shell', sub: 'unprivileged', delay: 0}
    - {type: wire, from: {x: 205, y: 150}, to: {x: 275, y: 150}, tone: danger, dashed: true, delay: 0.4}
    - {type: box, x: 275, y: 118, w: 175, h: 64, label: 'backup-tool', sub: 'SUID root', tone: danger, delay: 0.7}
    - {type: box, x: 470, y: 60, w: 150, h: 46, label: 'Defender', sub: 'the event lands', tone: accent, delay: 1.0}
    - {type: wire, from: {x: 450, y: 130}, to: {x: 545, y: 106}, tone: accent, dashed: true, delay: 1.2}
---
Your session drops you into an unprivileged shell. On the machine is a
`SUID`-root `backup-tool` that builds a command from your argument and hands it to
a shell - a command injection you can exploit for root.

Escalate from the `Terminal` - `$ backup-tool "x; id #"` - then
switch to the `Defender` tab. Somewhere in that live feed is the `commit_creds`
event your own escalation just caused - and the flag is printed on that row.
Your job is to find it.
