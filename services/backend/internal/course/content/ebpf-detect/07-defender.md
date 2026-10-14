---
kind: concept
ey: CONCEPT 5
eyt: The view is yours
chip:
  label: KEY
  tone: accent
title: The sensor runs in your lab, and streams to your Defender pane
aha: This is the flip that matters. eBPF is often something done to you by whoever owns the machine. Here it is yours - your kernel, your sensor, your feed - so you can learn detection by watching your own attack from the other side.
diagram:
  caption: your terminal on one side, the sensor's feed on the other
  shapes:
    - {type: box, x: 40, y: 116, w: 200, h: 68, label: 'Terminal', sub: 'you attack here', delay: 0}
    - {type: box, x: 380, y: 116, w: 200, h: 68, label: 'Defender', sub: 'you watch here', tone: accent, delay: 0.5}
    - {type: box, x: 210, y: 55, w: 200, h: 44, label: 'Tetragon', sub: 'in your microVM', delay: 0.8}
    - {type: wire, from: {x: 310, y: 99}, to: {x: 460, y: 116}, tone: accent, delay: 1.1}
---
The `eBPF` sensor in this lab is not someone else spying on you. `Tetragon` runs
inside your own throwaway microVM, watching your own kernel, and its live event
feed streams straight to the `Defender` pane beside your terminal.

So you get both seats at once: you attack in the `Terminal`, and you watch the
kernel react in the `Defender` tab. Learning to read that feed is the whole point
of the lab.
