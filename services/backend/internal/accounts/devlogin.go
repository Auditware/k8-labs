package accounts

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// FindOrCreateLocalUser backs the localhost-only credential login (see
// internal/api.devLogin, mounted only when DEV_LOGIN=1). It find-or-creates a
// user by a synthetic <username>@dev.local address and marks it admin, so a
// fresh local sign-in lands on a real lab with full catalog access. It never
// touches google_id/github_id - oauth_provider is "dev".
//
// This deliberately reintroduces, for local dev ONLY, the password path that
// config.go says was removed so dev exercises the same auth as prod. The
// tradeoff is accepted on purpose: prod never registers the route (DEV_LOGIN is
// unset there), so this code path cannot run outside a developer's machine.
func FindOrCreateLocalUser(ctx context.Context, pool *pgxpool.Pool, username string) (*User, error) {
	email := username + "@dev.local"

	if u, ok, err := findUser(ctx, pool, "email", email); err != nil {
		return nil, err
	} else if ok {
		// Keep the local account admin (so it always clears the grant gate) and
		// refresh last_login, matching what the OAuth paths do on return.
		if _, err := pool.Exec(ctx, `UPDATE users SET role = 'admin', last_login = now() WHERE id = $1`, u.ID); err != nil {
			return nil, err
		}
		u.Role = "admin"
		return u, nil
	}

	id, err := randomID()
	if err != nil {
		return nil, err
	}
	uname, err := uniqueUsername(ctx, pool, username)
	if err != nil {
		return nil, err
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, email, username, oauth_provider, role, created_at, last_login)
		 VALUES ($1, $2, $3, 'dev', 'admin', now(), now())`,
		id, email, uname,
	); err != nil {
		return nil, err
	}
	return &User{ID: id, Email: email, Username: uname, OAuthProvider: "dev", Role: "admin"}, nil
}
