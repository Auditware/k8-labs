#!/usr/bin/env bash
# LOCAL DEV ONLY: keep the local Vault unsealed automatically.
#
# Vault runs with Raft storage and seals on every pod restart. Prod keeps
# unsealing a manual,
# human-in-the-loop step BY DESIGN - the key shares are split across people and
# never all in one place. Locally none of that applies: there's a single keys
# file on disk (secrets/vault-unseal-keys.json, gitignored) and one operator, so
# a re-seal is pure friction - the memory-tight VM evicts the Vault pod, it comes
# back sealed, and the next fresh backend pod then hangs at its vault-agent init
# until someone runs the three unseal commands by hand.
#
# This watches for a sealed Vault and unseals it from the local keys file, so
# that interruption disappears. It is wired in as a Tilt local_resource
# (serve_cmd) - see the Tiltfile. NEVER run this against a real environment: it
# defeats the entire point of sealed storage.
set -uo pipefail

KEYS=secrets/vault-unseal-keys.json
NS=vault
POD=vault-0

sealed_state() {
  # Prints "true"/"false", or nothing if Vault isn't reachable yet.
  kubectl exec -n "$NS" "$POD" -- vault status -format=json 2>/dev/null \
    | python3 -c "import sys,json;print(str(json.load(sys.stdin).get('sealed')).lower())" 2>/dev/null
}

unseal_once() {
  command -v kubectl >/dev/null 2>&1 || { echo "vault-unseal: kubectl not found, skipping"; return 0; }
  [ -f "$KEYS" ] || { echo "vault-unseal: $KEYS not found, skipping"; return 0; }

  local sealed; sealed=$(sealed_state)
  [ "$sealed" = "true" ] || return 0   # unsealed, or not reachable yet - nothing to do

  echo "vault-unseal: Vault is sealed, applying 3 key shares"
  local key
  for key in $(python3 -c "import json;print('\n'.join(json.load(open('$KEYS'))['unseal_keys_b64'][:3]))" 2>/dev/null); do
    kubectl exec -n "$NS" "$POD" -- vault operator unseal "$key" >/dev/null 2>&1 || true
  done
  if [ "$(sealed_state)" = "false" ]; then
    echo "vault-unseal: unsealed"
  else
    echo "vault-unseal: WARN still sealed after applying keys - check $KEYS"
  fi
}

if [ "${1:-}" = "--watch" ]; then
  echo "vault-unseal: watching local Vault (dev convenience; prod stays manual)"
  while true; do unseal_once; sleep 15; done
else
  unseal_once
fi
