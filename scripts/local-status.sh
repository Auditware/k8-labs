#!/usr/bin/env bash
# Is local dev actually running the current code?
#
# The failure this guards against: Tilt dies, or its file-watcher goes stale, and
# the backend pod keeps serving an OLD binary with no signal - local silently
# falls hours behind HEAD (and behind prod). This makes that visible: it asks the
# running backend which commit it is (via the sha /healthz now reports) and
# compares it to HEAD. Run it any time local looks wrong; it also runs on
# `tilt up` (see the Tiltfile's local-status resource).
set -uo pipefail
CTX=k3d-k8labs-local
NS=default
head=$(git rev-parse --short HEAD 2>/dev/null || echo "?")
rc=0

# 1. Tilt alive? A zombie tilt (process up, API dead) is exactly what bit us -
#    connection refused on :10350 means it is down even if `ps` shows it running.
code=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:10350/api/view 2>/dev/null || echo 000)
if [ "$code" = "000" ]; then
  echo "✗ Tilt is NOT responding on :10350 (down or zombie). Restart it:  tilt down ; tilt up"
  rc=1
else
  echo "✓ Tilt responding on :10350"
fi

# 2. What commit is the running backend? Ask its own /healthz from inside the pod
#    (no host port-forward assumptions - PORT is the container's own env).
running=$(kubectl --context "$CTX" exec -n "$NS" deploy/backend-backend -c backend -- \
  sh -c 'wget -qO- "http://localhost:${PORT:-8080}/healthz"' 2>/dev/null \
  | sed -n 's/.*"sha":"\([^"]*\)".*/\1/p')
running=${running:-"?"}

echo "  HEAD:            $head"
echo "  backend running: $running"
if [ "$running" = "$head" ]; then
  echo "✓ local backend is current"
else
  echo "✗ DRIFT: backend is not running HEAD - Tilt likely isn't rebuilding. Restart Tilt."
  rc=1
fi
exit $rc
