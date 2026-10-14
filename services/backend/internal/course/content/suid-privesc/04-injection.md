---
kind: concept
ey: CONCEPT 3
eyt: The sloppy input
tag: the machinery
title: Handing user input to a shell is handing over the keyboard
aha: It is like reading a form aloud to an assistant who does whatever they hear. Write "Smith; burn the files" in the name field and they burn the files.
diagram:
  caption: the shell obeys punctuation, not intent
  shapes:
    - {type: box, x: 45, y: 116, w: 200, h: 60, label: 'cp YOURINPUT /backup/', sub: 'what the program meant', delay: 0}
    - {type: wire, from: {x: 245, y: 146}, to: {x: 325, y: 146}, tone: danger, delay: 0.5}
    - {type: box, x: 325, y: 106, w: 250, h: 80, label: 'cp x; id /backup/', sub: 'two commands, not one', tone: danger, delay: 0.8}
---
Programs often shell out - they build a command line as text and hand it to a
shell like `sh` to run. If any part of that text came from you and was not sanitized,
you do not just fill in a blank; you can end the intended command and start your
own. That is `command injection`, `CWE-78`.

The shell treats `;`, `|`, and `$(...)` as syntax. A program that builds a `cp`
command from your input - `cp <your input> /backup/` - and runs it through a
shell will, if you pass `x; id`, faithfully run the copy and then run `id`, as
whatever user the shell is running as.
