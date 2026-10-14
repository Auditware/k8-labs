---
kind: concept
ey: CONCEPT 4
eyt: From one command to a shell
tag: technique
title: A single injected command is enough to open a root shell
aha: You do not need to do everything in one line. You need to do one thing - start a shell. After that you just type, as root.
diagram:
  caption: injected once, root shell thereafter
  shapes:
    - {type: box, x: 55, y: 116, w: 210, h: 62, label: 'backup-tool "x; sh"', sub: 'inject a shell', tone: danger, delay: 0}
    - {type: wire, from: {x: 265, y: 147}, to: {x: 350, y: 147}, tone: danger, delay: 0.5}
    - {type: box, x: 350, y: 116, w: 170, h: 62, label: 'root shell', sub: 'uid 0', tone: danger, delay: 0.9}
---
The injected command can be anything a shell can run - including another shell.
Because `backup-tool` has already taken on the root identity by the time it runs
your input, the command runs as root, and a shell you start from it is a root
shell.

So you do not have to squeeze the whole attack into one injected command. Inject
a single `sh`, and you trade one one-shot command for an interactive root prompt
that you can type into freely.
