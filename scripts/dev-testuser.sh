#!/usr/bin/env bash
#
# Create a local, non-admin test user with an API key.
#
# Why this is a script and not startup seeding: the app used to seed an
# admin/password account at boot (adminSeed). That was removed so local dev
# exercises the same OAuth-only path as prod - and because the row it created
# outlived the toggle. Prod still carries an orphan admin account from when
# adminSeed was briefly enabled, months after the Secret was deleted. Seeding
# from inside the binary makes rows that survive the flag that made them; a
# script can't, because it isn't in the binary at all.
#
# What this gives you is a second account to verify the authorization boundary
# with: that a non-admin gets 403 on /admin/*, sees only granted labs, and gets
# an API spec with the admin routes stripped. Doing that by hand every time is
# how you get a subtly wrong test.
#
# The user CANNOT sign in through the browser - password login no longer
# exists, and this account has no Google/GitHub identity behind it. It's usable
# two ways: through its API key, and as a row in the admin Users page (to test
# grant/revoke/block/delete).
#
# Usage:
#   scripts/dev-testuser.sh                          # create + print an API key
#   scripts/dev-testuser.sh --grant ns-escape    # also grant a lab
#   scripts/dev-testuser.sh --delete                 # remove the user and its key
#
set -euo pipefail

CONTEXT="k3d-k8labs-local"
USER_ID="devtest01"
USERNAME="devtestuser"
EMAIL="devtest@localhost"

# Hard guard. This inserts users and API keys; pointed at a real environment it
# would create a live credential. The script refuses to run anywhere but the
# local k3d cluster, and deliberately passes --context explicitly on every call
# rather than trusting whatever the current context happens to be.
current="$(kubectl config current-context 2>/dev/null || echo none)"
if [ "$current" != "$CONTEXT" ]; then
  echo "refusing to run: current kubectl context is '$current', expected '$CONTEXT'." >&2
  echo "this script only ever targets the local dev cluster." >&2
  exit 1
fi

psql_exec() {
  kubectl exec -n default deploy/postgres-postgres --context "$CONTEXT" -- \
    sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tAc "$0"' "$1"
}

if [ "${1:-}" = "--delete" ]; then
  # api_keys and lab_grants both cascade from users.
  psql_exec "DELETE FROM users WHERE id = '$USER_ID';" >/dev/null
  echo "removed $USERNAME"
  exit 0
fi

grant_slug=""
if [ "${1:-}" = "--grant" ]; then
  grant_slug="${2:-}"
  if [ -z "$grant_slug" ]; then
    echo "--grant needs a module slug, e.g. --grant ns-escape" >&2
    exit 1
  fi
fi

# 32 bytes of randomness, same shape the app generates. Only the SHA-256 hash
# is stored - matching internal/accounts/apikeys.go, which is why the plaintext
# is printed here and nowhere else.
plaintext="k8labs_$(python3 -c 'import secrets; print(secrets.token_hex(32))')"
key_hash="$(python3 -c 'import hashlib,sys; print(hashlib.sha256(sys.argv[1].encode()).hexdigest())' "$plaintext")"
prefix="${plaintext:0:15}"
key_id="devkey$(python3 -c 'import secrets; print(secrets.token_hex(4))')"

psql_exec "
  INSERT INTO users (id, email, username, oauth_provider, role)
    VALUES ('$USER_ID', '$EMAIL', '$USERNAME', 'google', 'user')
    ON CONFLICT (id) DO NOTHING;
  INSERT INTO api_keys (id, user_id, name, prefix, key_hash)
    VALUES ('$key_id', '$USER_ID', 'dev test key', '$prefix', '$key_hash');
" >/dev/null

if [ -n "$grant_slug" ]; then
  psql_exec "
    INSERT INTO lab_grants (user_id, module_slug)
      VALUES ('$USER_ID', '$grant_slug')
      ON CONFLICT DO NOTHING;
  " >/dev/null
fi

grants="$(psql_exec "SELECT COALESCE(string_agg(module_slug, ', '), '(none)') FROM lab_grants WHERE user_id = '$USER_ID';" | tr -d '\r')"

cat <<EOF

  user      $USERNAME ($EMAIL)  role=user
  grants    $grants
  api key   $plaintext

  Shown once - the app stores only a hash. Re-run this script for a new key.

  Try it:
    curl -sk -H "X-API-Key: $plaintext" https://localhost:8443/api/v1/auth/me
    curl -sk -H "X-API-Key: $plaintext" https://localhost:8443/api/v1/modules
    curl -sk -o /dev/null -w '%{http_code}\\n' \\
      -H "X-API-Key: $plaintext" https://localhost:8443/api/v1/admin/users   # expect 403

  Clean up:
    scripts/dev-testuser.sh --delete

EOF
