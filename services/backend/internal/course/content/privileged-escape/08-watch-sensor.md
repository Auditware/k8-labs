---
kind: concept
ey: DETECTION
eyt: Now turn to the Defender
chip:
  label: DEFENDER
  tone: accent
title: An eBPF sensor in your lab saw you reach through PID 1
aha: Reaching the node felt like slipping past the wall unseen. But opening a path is a syscall, and a sensor watching the kernel reads the very path you asked for - and the /proc/1/root you typed is the escape written in plain sight.
revealLabel: where to look
diagram:
  caption: your open on the left, the sensor's event on the right
  shapes:
    - {type: box, x: 40, y: 116, w: 205, h: 68, label: 'you open /proc/1/root', sub: 'the node FS', tone: danger, delay: 0}
    - {type: box, x: 385, y: 116, w: 205, h: 68, label: 'Defender', sub: 'the open() event', tone: accent, delay: 0.6}
    - {type: box, x: 210, y: 55, w: 205, h: 44, label: 'Tetragon', sub: 'hooks openat in-kernel', delay: 0.9}
    - {type: wire, from: {x: 305, y: 99}, to: {x: 475, y: 116}, tone: accent, delay: 1.1}
---
Beside your terminal is a `Defender` pane. It streams live events from `Tetragon`,
an `eBPF` security sensor running inside your own throwaway lab - your kernel, your
sensor, your feed, not someone watching over your shoulder.

`Tetragon` hooks the `open()` syscall and posts an event whenever a process opens a
path beginning with `/proc/1/root`. That prefix is the breakout itself - asking the
kernel for `PID 1`'s root is asking to leave the container - so the read the shell
showed nothing for lands in the `Defender` feed with the exact path and the process
behind it. Break out, read the flag, then switch panes and find your own event.
