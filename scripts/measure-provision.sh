#!/usr/bin/env bash
#
# Measure click-to-terminal (provisioning) latency for the nested lab path.
#
# What it times, two ways:
#   - client-observed: POST /sessions -> first poll that sees status "running"
#     (what the student actually waits through before the terminal renders).
#   - backend per-phase: the `provision phase` slog lines that provisionNested
#     emits (pod-created / pod-ready / kubeconfig-fetched / nested-node-ready /
#     manifests-applied / foothold-ready) plus the total. This is the breakdown
#     that says WHICH gate dominates and whether a change actually moved it.
#
# Sessions run SERIALLY on purpose: the backend enforces one running session
# per user (409 otherwise), so each iteration is create -> wait running ->
# measure -> delete -> wait gone. Serial also keeps per-session timings clean.
#
# Local only. Guards on the k3d context exactly like dev-testuser.sh, and needs
# the Tilt stack up (the backend must run IN-cluster - the pm2 backend refuses
# to provision, see internal/labs/session.go's Create).
#
# Usage:
#   scripts/measure-provision.sh                 # 15 runs of ns-escape
#   N=20 MODULE=ns-escape scripts/measure-provision.sh
#
set -euo pipefail

CONTEXT="k3d-k8labs-local"
BASE="${BASE:-https://localhost:8443}"
MODULE="${MODULE:-ns-escape}"
N="${N:-15}"
NS="default"

# Same hard guard as dev-testuser.sh: never point this at a real environment.
current="$(kubectl config current-context 2>/dev/null || echo none)"
if [ "$current" != "$CONTEXT" ]; then
  echo "refusing to run: current kubectl context is '$current', expected '$CONTEXT'." >&2
  exit 1
fi

here="$(cd "$(dirname "$0")" && pwd)"

echo "== ensuring a granted test user + API key =="
# dev-testuser.sh prints the plaintext key once, on a line like: "  api key   k8labs_..."
KEY="$("$here/dev-testuser.sh" --grant "$MODULE" | awk '/api key/{print $3}')"
if [ -z "${KEY:-}" ]; then
  echo "failed to obtain an API key from dev-testuser.sh" >&2
  exit 1
fi

api() { curl -sk -H "X-API-Key: $KEY" "$@"; }

# Only parse backend logs produced from here on, so a re-run doesn't fold in an
# earlier run's phase lines. RFC3339, one second before now for clock skew.
SINCE="$(python3 -c 'import time;print(time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(time.time()-1)))')"

echo "== running $N serial $MODULE sessions against $BASE =="
ids=()
client_ms=()
for i in $(seq 1 "$N"); do
  t0="$(python3 -c 'import time;print(time.time())')"
  resp="$(api -X POST -H 'Content-Type: application/json' \
             -d "{\"module\":\"$MODULE\"}" "$BASE/api/v1/sessions")"
  id="$(printf '%s' "$resp" | python3 -c 'import sys,json;print(json.load(sys.stdin).get("id",""))' 2>/dev/null || true)"
  if [ -z "$id" ]; then
    echo "  run $i: create failed: $resp" >&2
    continue
  fi

  # Poll to running (or error). 0.25s cadence is finer than the frontend's 2s
  # so the harness itself isn't the thing adding latency to the measurement.
  status=""
  while :; do
    status="$(api "$BASE/api/v1/sessions/$id" | \
              python3 -c 'import sys,json;print(json.load(sys.stdin).get("status",""))' 2>/dev/null || true)"
    case "$status" in
      running) break ;;
      error|ended|gone) echo "  run $i ($id): ended as '$status'" >&2; break ;;
    esac
    sleep 0.25
  done
  t1="$(python3 -c 'import time;print(time.time())')"

  if [ "$status" = "running" ]; then
    ms="$(python3 -c "print(round(($t1-$t0)*1000))")"
    client_ms+=("$ms")
    ids+=("$id")
    printf '  run %2d  %-32s  client %6s ms\n' "$i" "$id" "$ms"
  fi

  # Free the per-user concurrency cap before the next iteration.
  api -X DELETE "$BASE/api/v1/sessions/$id" >/dev/null || true
  # Wait until it's actually gone so the next create isn't refused with a 409.
  for _ in $(seq 1 40); do
    gone="$(api -o /dev/null -w '%{http_code}' "$BASE/api/v1/sessions/$id")"
    [ "$gone" = "404" ] && break
    sleep 0.25
  done
done

echo
echo "== backend per-phase breakdown (from provisionNested slog) =="
POD="$(kubectl --context "$CONTEXT" -n "$NS" get pods --no-headers 2>/dev/null \
        | awk '/backend-backend/{print $1; exit}')"
if [ -z "$POD" ]; then
  echo "could not find backend pod for log harvest" >&2
else
  # Parser goes in a temp file rather than `python3 -c '...'` so it can use
  # whatever quotes it likes without fighting the shell's single-quoting.
  parser="$(mktemp -t measure-provision.XXXXXX).py"
  trap 'rm -f "$parser"' EXIT
  cat > "$parser" <<'PY'
import sys, json, os

phases = {}
totals = []
for line in sys.stdin:
    line = line.strip()
    if not line.startswith("{"):
        continue
    try:
        r = json.loads(line)
    except Exception:
        continue
    msg = r.get("msg")
    if msg == "provision phase":
        phases.setdefault(r["phase"], []).append(r["ms"])
    elif msg == "nested session running":
        totals.append(r["totalMs"])

def pct(xs, p):
    xs = sorted(xs)
    if not xs:
        return 0
    k = (len(xs) - 1) * p
    lo, hi = int(k), min(int(k) + 1, len(xs) - 1)
    return round(xs[lo] + (xs[hi] - xs[lo]) * (k - lo))

def row(label, xs):
    print(f"  {label:<22}{len(xs):>4}{pct(xs,0.5):>10}{pct(xs,0.95):>10}{max(xs):>10}")

order = ["pod-created", "pod-ready", "kubeconfig-fetched",
         "nested-node-ready", "manifests-applied", "foothold-ready"]
print(f"  {'phase':<22}{'n':>4}{'p50 ms':>10}{'p95 ms':>10}{'max ms':>10}")
for ph in order:
    if phases.get(ph):
        row(ph, phases[ph])
if totals:
    row("TOTAL (backend)", totals)

cm = [int(x) for x in os.environ.get("CLIENT_MS", "").split() if x]
if cm:
    print()
    row("CLIENT click->running", cm)
PY
  kubectl --context "$CONTEXT" -n "$NS" logs "$POD" -c backend --since-time="$SINCE" 2>/dev/null \
    | CLIENT_MS="${client_ms[*]:-}" python3 "$parser"
fi

echo
echo "done. clean up the test user with: scripts/dev-testuser.sh --delete"
