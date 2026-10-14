---
kind: concept
ey: RECAP
eyt: And the fix
chip:
  label: TAKEAWAY
  tone: accent
title: The repair is to never let input reach a shell
aha: This survives review because each piece looks reasonable. "It just backs up a file." "It just needs root to write there." Only the two together are fatal, and nobody reviews for the combination.
revealLabel: why it slips through
diagram:
  caption: same job, no shell, no borrowed root at your command
  shapes:
    - {type: box, x: 45, y: 66, w: 250, h: 58, label: 'cp <input> via /bin/sh', sub: 'before - shell parses it', tone: danger, delay: 0}
    - {type: wire, from: {x: 170, y: 124}, to: {x: 170, y: 168}, delay: 0.4}
    - {type: box, x: 45, y: 168, w: 250, h: 58, label: 'copy the file directly', sub: 'after - no shell involved', tone: accent, delay: 0.7}
    - {type: seal, cx: 430, cy: 148, r: 30, delay: 1.0}
    - {type: label, x: 430, y: 214, text: 'nothing to inject into', tone: accent, delay: 1.2}
---
The `backup-tool` never needed a shell. Copying a file is a direct operation -
open the source, write the destination - with no command line for a `;` to hide
in. Passing user input to `/bin/sh` was a convenience that turned a backup
utility into a root shell.

Two things to carry away. Treat every `SUID` binary as a place where your input
becomes root's input, and audit exactly what it does with what you control. And
whenever a program builds a command from data, ask where that data came from -
because if any of it came from a user, the shell will do what the user says.
