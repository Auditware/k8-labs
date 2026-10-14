package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// EnsureSchema creates the tables the app needs if they don't exist yet.
// Placeholder until a real migration tool (golang-migrate/goose/atlas) is
// chosen - still an open question. Fine for a single
// idempotent CREATE TABLE IF NOT EXISTS; not fine once schema changes need
// real up/down migrations.
func EnsureSchema(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			username TEXT NOT NULL UNIQUE,
			google_id TEXT UNIQUE,
			github_id TEXT UNIQUE,
			password_hash TEXT,
			oauth_provider TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			last_login TIMESTAMPTZ
		)
	`)
	if err != nil {
		return err
	}
	// Added after the table already existed in earlier runs - CREATE TABLE
	// IF NOT EXISTS above won't retrofit it. Same "no real migrations yet"
	// caveat as above.
	_, err = pool.Exec(ctx, `ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash TEXT`)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `ALTER TABLE users ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'user'`)
	if err != nil {
		return err
	}
	// Blocked users can't authenticate at all (see accounts.ErrUserBlocked) and
	// have their running sessions torn down at block time. Separate from role:
	// blocking preserves the row and its lab_grants, so unblocking restores
	// exactly what the user had before.
	_, err = pool.Exec(ctx, `ALTER TABLE users ADD COLUMN IF NOT EXISTS blocked BOOLEAN NOT NULL DEFAULT false`)
	if err != nil {
		return err
	}
	// Per-user daily quota override, in GiB-minutes. NULL (the default) means
	// "use the global LABS_DAILY_QUOTA_GIB_MINUTES"; a value here overrides it for
	// this one user - a higher ceiling for a power user, a tighter one for someone
	// overdoing it. 0 is a meaningful value (no allowance = cannot start labs),
	// distinct from NULL (fall back to the global default), which is why the
	// column is nullable rather than defaulting to 0.
	_, err = pool.Exec(ctx, `ALTER TABLE users ADD COLUMN IF NOT EXISTS daily_quota_gib_minutes DOUBLE PRECISION`)
	if err != nil {
		return err
	}
	// Lab entitlements. Registration is open (anyone with a Google/GitHub
	// account can sign up), but a new user starts with zero rows here and so
	// can see and start nothing until an admin grants a module explicitly.
	// Admins bypass this table entirely - see accounts.VisibleModules.
	//
	// module_slug is deliberately not a foreign key: the module catalog lives
	// in Go (internal/labs/registry.go), not in Postgres, so there's nothing to
	// reference yet. Once a Postgres catalog table exists this should
	// become a real FK.
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS lab_grants (
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			module_slug TEXT NOT NULL,
			granted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			granted_by TEXT REFERENCES users(id) ON DELETE SET NULL,
			PRIMARY KEY (user_id, module_slug)
		)
	`)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `CREATE INDEX IF NOT EXISTS lab_grants_user_id_idx ON lab_grants (user_id)`)
	if err != nil {
		return err
	}
	// Self-service profile fields (Settings -> General). All optional.
	//
	// The avatar is stored in Postgres as bytea rather than object storage
	// because there is no object storage in this stack (no MinIO/S3, see
	// docs/architecture/environments.md) and adding one for a handful of small
	// images isn't worth a new platform component. Bounded hard at
	// accounts.MaxAvatarBytes so the table and every backup can't grow without
	// limit - revisit if the user count or image size ever justifies real blob
	// storage.
	for _, stmt := range []string{
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS website TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS bio TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS x_url TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS linkedin_url TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar BYTEA`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_content_type TEXT`,
	} {
		if _, err = pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	// API keys. Only the hash is stored - the plaintext key is shown exactly
	// once, at creation, and is unrecoverable afterwards (see
	// accounts.CreateAPIKey). prefix is the short non-secret leading segment,
	// kept so the UI can identify a key in a list without holding the secret.
	//
	// A key authenticates as its owner and is authorized against that user's
	// current role and grants, so revoking a grant or blocking the user disables
	// their keys immediately. scopes lists the SENSITIVE capabilities a key is
	// explicitly authorized for (currently just drivers:write - uploading
	// research drivers). A scope-gated capability is denied to any key that does
	// not carry the scope by name, even one whose owner could do it in a browser;
	// it never widens beyond the owner's own access (the gated route still checks
	// the owner's entitlement). See accounts.ScopeDriversWrite / KeyHasScope.
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS api_keys (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			prefix TEXT NOT NULL,
			key_hash TEXT NOT NULL UNIQUE,
			scopes TEXT[] NOT NULL DEFAULT '{}',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			expires_at TIMESTAMPTZ,
			last_used_at TIMESTAMPTZ
		)
	`)
	if err != nil {
		return err
	}
	// scopes was added after api_keys shipped; ensure it exists on databases
	// created before then (there is no migration tool - see the file header).
	_, err = pool.Exec(ctx, `ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS scopes TEXT[] NOT NULL DEFAULT '{}'`)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `CREATE INDEX IF NOT EXISTS api_keys_user_id_idx ON api_keys (user_id)`)
	if err != nil {
		return err
	}
	// Every authenticated request using a key looks it up by hash, so this is
	// the hot path for API traffic.
	_, err = pool.Exec(ctx, `CREATE INDEX IF NOT EXISTS api_keys_key_hash_idx ON api_keys (key_hash)`)
	if err != nil {
		return err
	}
	// Course progress: which quiz answers a student has given, and whether the
	// course is complete.
	//
	// Keyed by (user, module) rather than by session on purpose. Sessions are
	// reaped in an hour; a student who runs out of time, restarts the lab
	// tomorrow, and finishes it has completed the course - tying progress to a
	// session would silently erase that.
	//
	// answers is jsonb keyed by step slug, and the step slug comes from the
	// filename minus its order prefix (see internal/course), so inserting a new
	// screen in the middle of a lesson renumbers files without invalidating
	// anyone's saved answers.
	//
	// started_at is stamped when the row is first created (the student's first
	// recorded answer, i.e. first engagement with the lab) and never moved.
	// solve_seconds is frozen at completion as the span started_at -> completed_at:
	// the leaderboard's "fastest escape" is the minimum of these across a user's
	// completed labs. It is deliberately elapsed wall-clock from first step to
	// finish, the only honest solve signal the progress record can back - not
	// active keyboard time, which nothing measures. It stays null for labs
	// completed before this tracking existed, which just excludes them from the
	// fastest-solve stat rather than inventing a number.
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS lab_progress (
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			module_slug TEXT NOT NULL,
			answers JSONB NOT NULL DEFAULT '{}'::jsonb,
			completed_at TIMESTAMPTZ,
			started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			solve_seconds INTEGER,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			PRIMARY KEY (user_id, module_slug)
		)
	`)
	if err != nil {
		return err
	}
	// Backfill the solve-time columns onto pre-existing lab_progress tables. On a
	// table that predates them, started_at defaults to now() for old rows (so
	// their solve_seconds stays null and they're simply not counted), and every
	// new row from here on records a real span.
	for _, stmt := range []string{
		`ALTER TABLE lab_progress ADD COLUMN IF NOT EXISTS started_at TIMESTAMPTZ NOT NULL DEFAULT now()`,
		`ALTER TABLE lab_progress ADD COLUMN IF NOT EXISTS solve_seconds INTEGER`,
	} {
		if _, err = pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	// Per-lab operator knobs, editable by an admin (Admin -> Lab Management).
	// Deliberately an OVERRIDE table, not a catalog: the module's shape (its
	// manifest bundle, entry pod) still lives in Go (internal/labs/registry.go),
	// and a slug with no row here just uses the built-in defaults. That keeps
	// this table small and makes "reset to default" a DELETE rather than a
	// guess at what the default was.
	//
	// Both durations are enforced by internal/labs' reaper, and duration_minutes
	// is what the lab UI counts down to - so a value here is a promise to the
	// student about how long their session lasts. It is not cosmetic.
	//
	// module_slug is deliberately not a foreign key, same reason as lab_grants:
	// there is no catalog table to reference yet.
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS lab_settings (
			module_slug TEXT PRIMARY KEY,
			duration_minutes INTEGER NOT NULL,
			idle_minutes INTEGER NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_by TEXT REFERENCES users(id) ON DELETE SET NULL
		)
	`)
	if err != nil {
		return err
	}
	// One row per finished session: what it cost, in raw components.
	//
	// The components are stored SEPARATELY and deliberately un-blended. It is
	// tempting to write a single "credits" number here, but that bakes today's
	// guess at the exchange rate into history permanently - and the weights are
	// exactly the thing there is no data to choose yet. Keeping the components
	// means the score is computed at read time and can be retuned, or made
	// per-tier, without losing or misreporting anything already recorded.
	//
	// gib_minutes is the primary unit because memory is what a session actually
	// denies to everyone else: it holds its whole reservation for its whole life
	// whether or not the student is typing (see labs.SessionMemory). An eBPF lab
	// therefore costs ~1.67x a normal one per minute, which is the real ratio,
	// not a policy choice.
	//
	// cpu_seconds and egress_bytes are recorded as 0 today. The columns exist now
	// because adding them later is a migration, and because both are wanted for
	// abuse detection rather than for capacity: CPU is time-shared and elastic,
	// so it says nothing about whether the node fits another session. Populating
	// them needs collectors that do not exist yet - node-level CPU accounting (the
	// kubelet stats API returns 500 while any Kata pod is running, so it cannot be
	// the source) and Cilium's per-pod egress counters.
	//
	// ended_reason distinguishes a student stopping deliberately from the reaper
	// taking a session on idle or on the hard cap, which is what tells you whether
	// the TTLs are set anywhere near right.
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS lab_usage (
			session_id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			module_slug TEXT NOT NULL,
			started_at TIMESTAMPTZ NOT NULL,
			ended_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			gib_minutes DOUBLE PRECISION NOT NULL,
			cpu_seconds DOUBLE PRECISION NOT NULL DEFAULT 0,
			egress_bytes BIGINT NOT NULL DEFAULT 0,
			ended_reason TEXT NOT NULL DEFAULT ''
		)
	`)
	if err != nil {
		return err
	}
	// The quota check runs on every session start and asks one question: how much
	// has this user consumed in the trailing window. That is exactly this index.
	_, err = pool.Exec(ctx, `CREATE INDEX IF NOT EXISTS lab_usage_user_ended_idx ON lab_usage (user_id, ended_at)`)
	if err != nil {
		return err
	}

	// research_drivers is the catalog for the kernel-driver research labs: a
	// research user uploads a compiled .ko, an admin vets it, and vetted drivers
	// can be selected and mounted into a session (see docs/architecture/
	// research-driver-catalog.md). The .ko bytes live in Postgres bytea for the
	// same reason avatars do - the stack has no object store and a module is small
	// (tens of KB to low single-digit MB, capped hard at drivers.MaxKoBytes). The
	// metadata columns are all EXTRACTED from the ELF at upload (never trusted from
	// the client); sha256 content-addresses the blob and dedups re-uploads.
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS research_drivers (
			id TEXT PRIMARY KEY,
			sha256 TEXT NOT NULL UNIQUE,
			filename TEXT NOT NULL,
			size_bytes INTEGER NOT NULL,
			ko_bytes BYTEA NOT NULL,
			mod_name TEXT NOT NULL DEFAULT '',
			vermagic TEXT NOT NULL DEFAULT '',
			license TEXT NOT NULL DEFAULT '',
			depends TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			is_signed BOOLEAN NOT NULL DEFAULT false,
			signer TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'pending',
			visibility TEXT NOT NULL DEFAULT 'private',
			uploaded_by TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			uploaded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			vetted_by TEXT REFERENCES users(id) ON DELETE SET NULL,
			vetted_at TIMESTAMPTZ,
			vet_reason TEXT NOT NULL DEFAULT ''
		)
	`)
	if err != nil {
		return err
	}
	// The user catalog query and the admin queue both filter by status; the
	// uploader index serves "my uploads" and the cascade on user delete.
	_, err = pool.Exec(ctx, `CREATE INDEX IF NOT EXISTS research_drivers_status_idx ON research_drivers (status)`)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `CREATE INDEX IF NOT EXISTS research_drivers_uploaded_by_idx ON research_drivers (uploaded_by)`)
	return err
}
