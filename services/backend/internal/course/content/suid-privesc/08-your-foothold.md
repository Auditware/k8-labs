---
kind: concept
ey: THE LAB
eyt: Your starting position
tag: go and look
title: Here is what you have been given
aha: Everything you need is already on the machine. The work is not to find a way in - you are in. The work is to notice which program trusts you more than it should.
revealLabel: how to start
diagram:
  caption: one of these you own; one runs as root
  shapes:
    - {type: box, x: 55, y: 116, w: 180, h: 64, label: 'your shell', sub: 'uid 10001', delay: 0}
    - {type: box, x: 300, y: 70, w: 200, h: 58, label: 'backup-tool', sub: 'SUID root', tone: warn, delay: 0.4}
    - {type: box, x: 300, y: 160, w: 200, h: 58, label: 'the flag', sub: 'root reads only', tone: danger, delay: 0.7}
    - {type: wire, from: {x: 235, y: 140}, to: {x: 300, y: 99}, tone: danger, dashed: true, delay: 1.0}
    - {type: wire, from: {x: 500, y: 99}, to: {x: 500, y: 160}, tone: danger, dashed: true, delay: 1.2}
---
Your session drops you into a shell as an unprivileged user. Somewhere on the
machine is a program called `backup-tool` that is `SUID` root, and a flag file
that only root can read.

Start by looking at `backup-tool`: `$ ls -l /usr/local/bin/backup-tool` shows the SUID bit,
and `$ cat /app/flag.txt` shows the flag is out of your reach for now. The tool wraps
your argument in `cp <arg> /backup/` and runs it through a shell, so you end the copy
with `;`, run your own command, and close with `#` to comment out the `/backup/` it
staples on the end: `$ backup-tool "x; id #"` runs `id` as root and prints `uid=0`.
Now do the same with the read that was refused a moment ago, and the flag is yours.
