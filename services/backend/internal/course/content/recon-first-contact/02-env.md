---
kind: concept
ey: CONCEPT 2
eyt: Reading the environment
tag: technique
title: env is the first thing an attacker reads
aha: Applications are configured through their environment, and configuration is where secrets leak. One command prints all of it.
diagram:
  caption: one command prints every variable the pod was started with
  shapes:
    - {type: box, x: 60, y: 116, w: 150, h: 64, label: 'env', sub: 'one command', delay: 0}
    - {type: wire, from: {x: 210, y: 148}, to: {x: 320, y: 148}, delay: 0.4}
    - {type: box, x: 320, y: 116, w: 260, h: 64, label: 'every variable', sub: 'hostnames, flags, secrets', tone: accent, delay: 0.7}
---
Run `$ env`. It prints every environment variable the pod was started with - and
applications are configured almost entirely through it: hostnames, feature
flags, and far too often, credentials.

Look past the noise (`APP_ENV`, `LOG_LEVEL`) for anything that looks like a
password, a key, or a connection string. A value injected from a `Secret` shows
up here in cleartext, indistinguishable from a plain string. Nothing on this
screen touched the network; you are just reading your own process.
