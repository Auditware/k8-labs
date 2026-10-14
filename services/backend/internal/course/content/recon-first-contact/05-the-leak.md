---
kind: concept
ey: CONCEPT 4
eyt: The finding
chip: {label: the leak, tone: danger}
title: The pod already leaked a secret to you
aha: A Secret consumed as an environment variable is a Secret printed in cleartext to every process in the pod. The boundary it was supposed to provide is gone.
diagram:
  caption: a Secret consumed as env is a Secret in cleartext
  shapes:
    - {type: box, x: 40, y: 116, w: 150, h: 64, label: 'Secret', sub: 'orders-db', tone: accent, delay: 0}
    - {type: wire, from: {x: 190, y: 148}, to: {x: 300, y: 148}, tone: danger, dashed: true, delay: 0.4}
    - {type: box, x: 300, y: 116, w: 130, h: 64, label: 'env var', sub: 'cleartext', tone: danger, delay: 0.7}
    - {type: wire, from: {x: 430, y: 148}, to: {x: 500, y: 148}, tone: danger, delay: 0.9}
    - {type: box, x: 500, y: 116, w: 110, h: 64, label: 'you', sub: 'read it', delay: 1.1}
---
That database password in `env` did not belong there. It came from a `Secret`,
wired into the container through a `secretKeyRef` - Kubernetes read the Secret
and set its value as an environment variable when the pod started.

The moment a `Secret` is consumed that way it stops being protected: it sits in
`env`, in `/proc/<pid>/environ`, and in the environment of every child process
you spawn. You did not break the Secret's protection - the pod spec gave it
away. That leaked value is your flag.
