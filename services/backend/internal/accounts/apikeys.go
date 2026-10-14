package accounts

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// KeyPrefix marks our keys so a leaked one is recognisable on sight - in a log,
// a paste, or a secret scanner. Same reasoning as GitHub's ghp_ / Stripe's sk_.
const KeyPrefix = "k8labs_"

// DefaultKeyTTL is how long a key lasts when the caller doesn't choose. Keys
// can be created without an expiry, but the UI defaults to this: an agent key
// that leaks and is never noticed stops working eventually rather than never.
const DefaultKeyTTL = 30 * 24 * time.Hour

// ErrInvalidAPIKey covers every failure to authenticate a key - malformed,
// unknown, or expired. Deliberately one error: telling a caller which of those
// it was helps an attacker enumerate valid keys and helps a legitimate user not
// at all.
var ErrInvalidAPIKey = errors.New("invalid api key")

// Scope values a key may carry. A scope NARROWS a key to a capability it must
// explicitly hold; it never grants anything the owner lacks. The route enforcing
// a scope also re-checks the owner's entitlement, so a scoped key is the owner's
// access intersected with its scopes.
const (
	// ScopeDriversWrite lets a key upload research drivers (POST /drivers). Only
	// mintable by a user who has research-lab access; the upload route requires
	// both this scope on the key and research access on the owner.
	ScopeDriversWrite = "drivers:write"
)

// validScopes is the closed set a key may be created with. Anything else is
// rejected at creation so a typo'd or forged scope can never sit unnoticed on a
// key.
var validScopes = map[string]bool{
	ScopeDriversWrite: true,
}

// ValidAPIKeyScope reports whether s is a scope a key may carry.
func ValidAPIKeyScope(s string) bool { return validScopes[s] }

// APIKey is the metadata safe to show a user. The secret itself is never in
// here - it exists only in the CreateAPIKey return value, once.
type APIKey struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Scopes     []string   `json:"scopes"`
	CreatedAt  time.Time  `json:"createdAt"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
}

// hashKey is SHA-256, not bcrypt, and that's deliberate. bcrypt's cost is there
// to slow down guessing a low-entropy human password; an API key here is 32
// bytes from crypto/rand, which is not guessable at any cost factor. Meanwhile
// every API request has to verify one, so a deliberately slow hash would just
// be a self-inflicted rate limit. Same reasoning GitHub and Stripe apply to
// their tokens.
func hashKey(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// CreateAPIKey generates a key, stores only its hash, and returns the
// plaintext exactly once. There is no way to recover it afterwards - the caller
// must show it to the user immediately or it's lost.
//
// expiresAt nil means the key never expires. scopes NARROWS the key to those
// capabilities; nil/empty is a full-power key. Every scope must be known
// (ValidAPIKeyScope) - the caller is responsible for also checking the owner is
// entitled to any scope it grants.
func CreateAPIKey(ctx context.Context, pool *pgxpool.Pool, userID, name string, expiresAt *time.Time, scopes []string) (*APIKey, string, error) {
	scopes = normalizeScopes(scopes)
	for _, s := range scopes {
		if !ValidAPIKeyScope(s) {
			return nil, "", fmt.Errorf("unknown api key scope %q", s)
		}
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, "", err
	}
	secret := hex.EncodeToString(raw)
	plaintext := KeyPrefix + secret

	id, err := randomID()
	if err != nil {
		return nil, "", err
	}

	// Non-secret leading segment, purely so the UI can say "which key is this"
	// in a list. Short enough to be useless to an attacker on its own.
	prefix := plaintext[:len(KeyPrefix)+8]

	if strings.TrimSpace(name) == "" {
		name = "unnamed key"
	}

	_, err = pool.Exec(ctx,
		`INSERT INTO api_keys (id, user_id, name, prefix, key_hash, scopes, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id, userID, name, prefix, hashKey(plaintext), scopes, expiresAt)
	if err != nil {
		return nil, "", err
	}

	return &APIKey{ID: id, Name: name, Prefix: prefix, Scopes: scopes, CreatedAt: time.Now().UTC(), ExpiresAt: expiresAt}, plaintext, nil
}

// normalizeScopes drops blanks and de-dupes, and returns a non-nil slice so it
// encodes as [] and stores as an empty array rather than NULL.
func normalizeScopes(scopes []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, s := range scopes {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// ListAPIKeys returns a user's keys, newest first. Metadata only - the secrets
// aren't stored, so they can't be listed even by accident.
func ListAPIKeys(ctx context.Context, pool *pgxpool.Pool, userID string) ([]APIKey, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, name, prefix, scopes, created_at, expires_at, last_used_at
		 FROM api_keys WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := []APIKey{}
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(&k.ID, &k.Name, &k.Prefix, &k.Scopes, &k.CreatedAt, &k.ExpiresAt, &k.LastUsedAt); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// DeleteAPIKey revokes a key. Scoped by user_id as well as id so one user can't
// revoke another's key by guessing an ID - the handler's auth check says who
// you are, this makes sure the row you're deleting is actually yours.
//
// Reports whether a row was actually removed. The caller turns "no" into a 404,
// because answering 200 for a key that wasn't yours (or didn't exist) tells a
// caller their revocation succeeded when nothing happened - actively dangerous
// for a revoke, where the whole point is being certain the credential is dead.
// Returning the same 404 for "no such key" and "not your key" is deliberate:
// it's honest to the owner without confirming another user's key IDs.
func DeleteAPIKey(ctx context.Context, pool *pgxpool.Pool, userID, keyID string) (bool, error) {
	tag, err := pool.Exec(ctx, `DELETE FROM api_keys WHERE id = $1 AND user_id = $2`, keyID, userID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// AuthenticateAPIKey resolves a plaintext key to its owner and the key's scopes,
// or ErrInvalidAPIKey. An empty scopes slice means a full-power key.
//
// The returned user is read fresh from the users table on every call, which is
// what makes a key track its owner's current access: blocking the user or
// revoking a grant takes effect on the very next request, with no key state to
// update. A blocked owner's key is rejected here outright.
func AuthenticateAPIKey(ctx context.Context, pool *pgxpool.Pool, plaintext string) (*User, []string, error) {
	if !strings.HasPrefix(plaintext, KeyPrefix) {
		return nil, nil, ErrInvalidAPIKey
	}

	var (
		keyID      string
		userID     string
		expiresAt  *time.Time
		storedHash string
		scopes     []string
	)
	err := pool.QueryRow(ctx,
		`SELECT id, user_id, expires_at, key_hash, scopes FROM api_keys WHERE key_hash = $1`,
		hashKey(plaintext),
	).Scan(&keyID, &userID, &expiresAt, &storedHash, &scopes)
	if err == pgx.ErrNoRows {
		return nil, nil, ErrInvalidAPIKey
	}
	if err != nil {
		return nil, nil, err
	}

	// The lookup above already matched on the hash, so this can't fail in
	// practice; it's here so the comparison that decides authentication is a
	// constant-time one rather than the database's own equality operator.
	if subtle.ConstantTimeCompare([]byte(storedHash), []byte(hashKey(plaintext))) != 1 {
		return nil, nil, ErrInvalidAPIKey
	}

	if expiresAt != nil && time.Now().After(*expiresAt) {
		return nil, nil, ErrInvalidAPIKey
	}

	u, found, err := findUser(ctx, pool, "id", userID)
	if err != nil {
		return nil, nil, err
	}
	if !found || u.Blocked {
		return nil, nil, ErrInvalidAPIKey
	}

	// Best-effort: a failure to record usage must not fail the request. Used
	// only to show "last used" in the UI so a stale key is easy to spot.
	_, _ = pool.Exec(ctx, `UPDATE api_keys SET last_used_at = now() WHERE id = $1`, keyID)

	return u, scopes, nil
}

// KeyHasScope reports whether a key explicitly carries a scope. Scopes gate
// SENSITIVE capabilities that a key must be authorized for by name: an empty set
// (a general full-power key) does NOT satisfy any scope, so those capabilities
// are denied by default and only a key minted with the scope may use them. This
// is deliberately stricter than "empty means all" - driver upload must be an
// opt-in per key, not something every one of a user's keys can do.
func KeyHasScope(scopes []string, want string) bool {
	for _, s := range scopes {
		if s == want {
			return true
		}
	}
	return false
}
