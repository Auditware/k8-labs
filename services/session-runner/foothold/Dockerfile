# syntax=docker/dockerfile:1

# The standard foothold image for nested (namespace/RBAC-escape) modules: a
# minimal shell + kubectl + a couple of exploitation staples. Deliberately tiny
# (~60 MB vs alpine/k8s's ~850 MB) because it's baked into the session-runner's
# airgap dir (see ../Dockerfile) and imported into every nested cluster on boot
# - so it never pulls at session time (near-instant starts) and needs no public
# registry. Modules that just need "a shell with kubectl" reference this image
# with imagePullPolicy: Never (see internal/labs/modules/*.yaml).
FROM alpine:3.20
ARG TARGETARCH=amd64
ARG KUBECTL_VERSION=v1.31.5
RUN apk add --no-cache ca-certificates bash curl jq \
 && curl -sSLo /usr/local/bin/kubectl "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/${TARGETARCH}/kubectl" \
 && chmod +x /usr/local/bin/kubectl
# A named account for uid/gid 10001, the unprivileged `user` labs run their
# shell as (e.g. suid-privesc's `shell` container: runAsUser/runAsGroup 10001).
# Without a passwd entry the shell's `\u` in PS1 resolves to empty, so the
# terminal prompt renders as a broken-looking "@host" (see internal/labs/
# terminal.go). Naming the account makes the prompt read "user@host", and -
# the point in a privesc lab - "root@host" the instant an escalation lands.
# Purely cosmetic: same uid, same gid, same (lack of) privileges. Labs that run
# the foothold as root (ns-escape) already have root's passwd entry.
RUN addgroup -g 10001 user \
 && adduser -D -u 10001 -G user -s /bin/sh user
CMD ["sleep", "infinity"]
