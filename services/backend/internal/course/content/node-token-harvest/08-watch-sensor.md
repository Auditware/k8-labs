---
kind: concept
ey: DETECTION
eyt: Now turn to the Defender
chip:
  label: DEFENDER
  tone: accent
title: An eBPF sensor in your lab saw the token leave the disk
aha: The harvest felt silent - no API call as yourself, just a file read on a node you happened to have mounted. But a read is a syscall, and a sensor watching the kernel sees every one. Silent to the shell is not silent to the kernel.
revealLabel: where to look
diagram:
  caption: your read on the left, the sensor's event on the right
  shapes:
    - {type: box, x: 40, y: 116, w: 200, h: 68, label: 'you read the token', sub: 'off /host', tone: danger, delay: 0}
    - {type: box, x: 380, y: 116, w: 210, h: 68, label: 'Defender', sub: 'the open() event', tone: accent, delay: 0.6}
    - {type: box, x: 205, y: 55, w: 210, h: 44, label: 'Tetragon', sub: 'hooks openat in-kernel', delay: 0.9}
    - {type: wire, from: {x: 300, y: 99}, to: {x: 470, y: 116}, tone: accent, delay: 1.1}
---
Beside your terminal is a `Defender` pane. It streams live events from `Tetragon`,
an `eBPF` security sensor running inside your own throwaway lab - your kernel, your
sensor, your feed, not someone watching over your shoulder.

`Tetragon` hooks the `open()` syscall in the kernel and posts an event whenever a
process opens a kubelet `token` file through the node mount. So the moment you
`cat` a token under `/host/var/lib/kubelet/pods`, the read your own account could
never be blamed for shows up in the `Defender` feed - the file path, and the
process that reached for it. Run the harvest, then switch panes and find your own
event.
