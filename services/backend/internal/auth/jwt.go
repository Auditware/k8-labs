// Package auth issues and verifies the stateless session JWTs. The signing
// key is injected by Vault's Agent Injector in-cluster
// - this package just reads it from the environment, it doesn't know or care
// where that value came from.
package auth

import (
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrNoSigningKey = errors.New("JWT_SIGNING_KEY not set")

// ScopeAI marks a token minted for the AI lab-assist grant. It is deliberately
// NOT a normal login/session token: requireAuth rejects any token carrying this
// scope, so an AI key can only ever reach the dedicated /ai/* routes that check
// it explicitly (see internal/api/ai.go). This is the deterministic confinement
// that keeps a leaked AI key off every user endpoint - flag submission included.
const ScopeAI = "ai"

// IssueAIToken mints a scoped, short-lived token for the AI lab-assist grant. It
// binds to one session (sid) and carries a unique id (jti) so the server can
// revoke it out-of-band (lab closed, user closed the panel) before its exp - the
// live grant, not the JWT exp alone, is what the /ai/* handlers check. scope is
// ScopeAI, which every non-/ai route refuses.
func IssueAIToken(userID, sessionID, jti string, ttl time.Duration) (string, error) {
	key, err := signingKey()
	if err != nil {
		return "", err
	}
	claims := jwt.MapClaims{
		"sub":   userID,
		"sid":   sessionID,
		"jti":   jti,
		"scope": ScopeAI,
		"exp":   time.Now().Add(ttl).Unix(),
		"iat":   time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(key)
}

// Scope returns the token's scope claim, or "" if unset - used by requireAuth to
// refuse ScopeAI tokens on the normal API surface.
func Scope(claims jwt.MapClaims) string {
	s, _ := claims["scope"].(string)
	return s
}

func signingKey() ([]byte, error) {
	key := os.Getenv("JWT_SIGNING_KEY")
	if key == "" {
		return nil, ErrNoSigningKey
	}
	return []byte(key), nil
}

// IssueSessionToken mints a token scoped to one lab session, expiring with
// its TTL - matches the CronJob that deletes the underlying microVM.
func IssueSessionToken(userID, sessionID string, ttl time.Duration) (string, error) {
	key, err := signingKey()
	if err != nil {
		return "", err
	}
	claims := jwt.MapClaims{
		"sub": userID,
		"sid": sessionID,
		"exp": time.Now().Add(ttl).Unix(),
		"iat": time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(key)
}

// IssueLoginToken mints the general "you are logged in" session token set
// as the auth cookie after Google/GitHub sign-in - distinct from
// IssueSessionToken, which scopes to one specific lab session instead.
func IssueLoginToken(userID, email string, ttl time.Duration) (string, error) {
	key, err := signingKey()
	if err != nil {
		return "", err
	}
	claims := jwt.MapClaims{
		"sub":   userID,
		"email": email,
		"exp":   time.Now().Add(ttl).Unix(),
		"iat":   time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(key)
}

func VerifyToken(raw string) (jwt.MapClaims, error) {
	key, err := signingKey()
	if err != nil {
		return nil, err
	}
	token, err := jwt.Parse(raw, func(t *jwt.Token) (interface{}, error) {
		return key, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !token.Valid {
		return nil, errors.New("invalid or expired token")
	}
	return token.Claims.(jwt.MapClaims), nil
}
