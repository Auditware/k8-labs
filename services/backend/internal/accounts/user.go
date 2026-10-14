package accounts

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Username bounds and shape. The handle is a public identity: it's shown on the
// leaderboard and is the path segment in /operators/{username}, so it has to be
// URL-safe and can't be an unbounded string. Letters/digits with interior _ or
// -, no leading or trailing separator, 3-32 chars.
const (
	MinUsernameLen = 3
	MaxUsernameLen = 32
)

var usernameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{1,30}[A-Za-z0-9]$`)

var (
	// ErrInvalidUsername is a shape/length rejection - safe to show the user.
	ErrInvalidUsername = errors.New("display name must be 3-32 characters: letters, digits, _ or -")
	// ErrUsernameTaken is a uniqueness rejection. The handle is UNIQUE in the DB
	// and is matched case-insensitively here so "Alice" can't shadow "alice".
	ErrUsernameTaken = errors.New("that display name is already taken")
)

// validateUsername enforces the handle's shape and length. Pure so it can be
// tested without a DB; the caller trims first. This is the URL-safety half of
// the rule - uniqueness is checked separately in UpdateUsername.
func validateUsername(username string) error {
	if len(username) < MinUsernameLen || len(username) > MaxUsernameLen || !usernameRe.MatchString(username) {
		return ErrInvalidUsername
	}
	return nil
}

// UpdateUsername validates and sets the caller's own handle, returning the
// stored value. Uniqueness is checked case-insensitively up front, and the DB's
// UNIQUE constraint is the backstop for the check-then-write race (a concurrent
// claim surfaces as a 23505 we translate to ErrUsernameTaken, not a 500).
func UpdateUsername(ctx context.Context, pool *pgxpool.Pool, userID, username string) (string, error) {
	username = strings.TrimSpace(username)
	if err := validateUsername(username); err != nil {
		return "", err
	}

	// Case-insensitive collision check, excluding the caller's own row so
	// re-saving an unchanged name (or a mere case change of it) isn't rejected as
	// taken.
	var exists bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE lower(username) = lower($1) AND id <> $2)`,
		username, userID,
	).Scan(&exists); err != nil {
		return "", err
	}
	if exists {
		return "", ErrUsernameTaken
	}

	if _, err := pool.Exec(ctx,
		`UPDATE users SET username = $1 WHERE id = $2`, username, userID); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return "", ErrUsernameTaken
		}
		return "", err
	}
	return username, nil
}

type User struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	Username      string `json:"username"`
	OAuthProvider string `json:"oauthProvider"`
	// Role is "user" or "admin" - see internal/api/auth.go's requireAdmin,
	// which checks it fresh from the DB rather than trusting a JWT claim,
	// so revoking admin access takes effect without waiting for a token
	// to expire.
	Role string `json:"role"`
	// Blocked users can't authenticate - checked at login (see
	// ErrUserBlocked) and re-checked on every request by requireAuth, for the
	// same reason Role is re-read from the DB: blocking must take effect
	// immediately, not whenever an already-issued token happens to expire.
	Blocked bool `json:"blocked"`
	// HasAvatar is `avatar IS NOT NULL` - it lets any client that has the
	// current user (the whole frontend, via /auth/me) decide whether to show
	// the uploaded image at /users/{id}/avatar or fall back to an initial,
	// without a second round-trip to /profile. The image bytes are never put
	// on this struct; see accounts.GetProfile for the same NULL-check pattern.
	HasAvatar bool `json:"hasAvatar"`
}

// FindOrCreateGoogleUser mirrors our standard find-or-create logic: match by google_id first,
// fall back to matching by email (linking the provider to an
// already-known account), otherwise create a new user.
func FindOrCreateGoogleUser(ctx context.Context, pool *pgxpool.Pool, gu *GoogleUser) (*User, error) {
	if u, ok, err := findUser(ctx, pool, "google_id", gu.Sub); err != nil || ok {
		if err == nil {
			err = touchLastLogin(ctx, pool, u.ID)
		}
		return u, err
	}

	if u, ok, err := findUser(ctx, pool, "email", gu.Email); err != nil {
		return nil, err
	} else if ok {
		_, err := pool.Exec(ctx,
			`UPDATE users SET google_id = $1, oauth_provider = 'google', last_login = now() WHERE id = $2`,
			gu.Sub, u.ID)
		return u, err
	}

	baseUsername := gu.Name
	if baseUsername == "" {
		baseUsername = gu.GivenName
	}
	if baseUsername == "" {
		baseUsername = usernameFromEmail(gu.Email)
	}
	return createUser(ctx, pool, gu.Email, baseUsername, "google_id", gu.Sub)
}

func FindOrCreateGitHubUser(ctx context.Context, pool *pgxpool.Pool, ghu *GitHubUser) (*User, error) {
	githubID := strconv.FormatInt(ghu.ID, 10)

	if u, ok, err := findUser(ctx, pool, "github_id", githubID); err != nil || ok {
		if err == nil {
			err = touchLastLogin(ctx, pool, u.ID)
		}
		return u, err
	}

	if u, ok, err := findUser(ctx, pool, "email", ghu.Email); err != nil {
		return nil, err
	} else if ok {
		_, err := pool.Exec(ctx,
			`UPDATE users SET github_id = $1, oauth_provider = 'github', last_login = now() WHERE id = $2`,
			githubID, u.ID)
		return u, err
	}

	baseUsername := ghu.Name
	if baseUsername == "" {
		baseUsername = ghu.Login
	}
	return createUser(ctx, pool, ghu.Email, baseUsername, "github_id", githubID)
}

// findUser is a pure lookup - callers decide whether a match means "this is
// a login, touch last_login" (direct provider-ID match) or "about to link a
// new provider to an existing account" (email match, the subsequent UPDATE
// sets last_login itself).
func findUser(ctx context.Context, pool *pgxpool.Pool, column, value string) (*User, bool, error) {
	if value == "" {
		return nil, false, nil
	}
	var u User
	err := pool.QueryRow(ctx,
		`SELECT id, email, username, oauth_provider, role, blocked, avatar IS NOT NULL FROM users WHERE `+column+` = $1`,
		value,
	).Scan(&u.ID, &u.Email, &u.Username, &u.OAuthProvider, &u.Role, &u.Blocked, &u.HasAvatar)
	if err == pgx.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &u, true, nil
}

func touchLastLogin(ctx context.Context, pool *pgxpool.Pool, id string) error {
	_, err := pool.Exec(ctx, `UPDATE users SET last_login = now() WHERE id = $1`, id)
	return err
}

func createUser(ctx context.Context, pool *pgxpool.Pool, email, baseUsername, providerCol, providerID string) (*User, error) {
	id, err := randomID()
	if err != nil {
		return nil, err
	}
	username, err := uniqueUsername(ctx, pool, baseUsername)
	if err != nil {
		return nil, err
	}
	provider := strings.TrimSuffix(providerCol, "_id")

	_, err = pool.Exec(ctx,
		`INSERT INTO users (id, email, username, `+providerCol+`, oauth_provider, created_at, last_login)
		 VALUES ($1, $2, $3, $4, $5, now(), now())`,
		id, email, username, providerID, provider,
	)
	if err != nil {
		return nil, err
	}
	return &User{ID: id, Email: email, Username: username, OAuthProvider: provider, Role: "user"}, nil
}

func uniqueUsername(ctx context.Context, pool *pgxpool.Pool, base string) (string, error) {
	if base == "" {
		base = "user"
	}
	username := base
	for suffix := 2; suffix < 9999; suffix++ {
		var count int
		err := pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE username = $1`, username).Scan(&count)
		if err != nil {
			return "", err
		}
		if count == 0 {
			return username, nil
		}
		username = base + strconv.Itoa(suffix)
	}
	return "", pgx.ErrNoRows
}

func usernameFromEmail(email string) string {
	if at := strings.IndexByte(email, '@'); at >= 0 {
		return email[:at]
	}
	return "user"
}

func randomID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
