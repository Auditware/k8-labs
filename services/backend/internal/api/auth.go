package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"backend/internal/accounts"
	"backend/internal/auth"
)

const sessionTTL = 7 * 24 * time.Hour

type ctxKey string

const userIDKey ctxKey = "userID"

// userKey holds the *accounts.User requireAuth loaded for this request. Kept
// alongside userIDKey rather than replacing it so the many handlers that only
// need the ID stay unchanged.
const userKey ctxKey = "user"

// apiKeyScopesKey holds the scopes of the API key this request authenticated
// with. Set ONLY on the API-key auth path; absent for cookie/JWT (browser)
// sessions. A handler distinguishes the two with apiKeyScopes: a browser session
// is unrestricted, while an API-key request is limited to what its key carries.
const apiKeyScopesKey ctxKey = "apiKeyScopes"

// isHTTPS looks at what Traefik (or the local Vite proxy) tells us about
// the original request - this process itself only ever speaks plain HTTP,
// TLS is terminated upstream.
func isHTTPS(r *http.Request) bool {
	return r.Header.Get("X-Forwarded-Proto") == "https"
}

func authCookie(r *http.Request, token string) *http.Cookie {
	c := &http.Cookie{
		Name:     "token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   int(sessionTTL.Seconds()),
	}
	if isHTTPS(r) {
		c.SameSite = http.SameSiteStrictMode
		c.Secure = true
	} else {
		c.SameSite = http.SameSiteLaxMode
	}
	return c
}

func clearAuthCookie(r *http.Request) *http.Cookie {
	c := authCookie(r, "")
	c.MaxAge = -1
	return c
}

// googleCallback godoc
// @Summary     Google OAuth callback
// @Description Exchanges an authorization code for the Google account, finds or creates the matching user, and issues a session (cookie + Bearer token, see finishLogin).
// @Tags        auth
// @Accept      json
// @Produce     json
// @Param       body body object true "{\"code\": \"...\", \"redirect_uri\": \"...\"}"
// @Success     200 {object} object "{\"user\": accounts.User, \"token\": string}"
// @Failure     400 {string} string "invalid request body or redirect_uri not allowed"
// @Failure     401 {string} string "code exchange or userinfo lookup failed"
// @Router      /auth/google/callback [post]
func googleCallback(cfg accounts.Config, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Code        string `json:"code"`
			RedirectURI string `json:"redirect_uri"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if !cfg.IsAllowedRedirect(body.RedirectURI) {
			slog.Warn("auth fail", "method", "google", "reason", "redirect_uri_not_allowed", "redirect_uri", body.RedirectURI)
			http.Error(w, "redirect_uri not allowed", http.StatusBadRequest)
			return
		}

		accessToken, err := cfg.ExchangeGoogleCode(r.Context(), body.Code, body.RedirectURI)
		if err != nil {
			slog.Warn("auth fail", "method", "google", "reason", "code_exchange_failed", "error", err)
			http.Error(w, "google code exchange failed", http.StatusUnauthorized)
			return
		}
		googleUser, err := cfg.GoogleUserInfo(r.Context(), accessToken)
		if err != nil {
			slog.Warn("auth fail", "method", "google", "reason", "userinfo_failed", "error", err)
			http.Error(w, "google userinfo failed", http.StatusUnauthorized)
			return
		}

		user, err := accounts.FindOrCreateGoogleUser(r.Context(), pool, googleUser)
		if err != nil {
			slog.Error("auth fail", "method", "google", "reason", "find_or_create_user_failed", "error", err)
			http.Error(w, "failed to find or create user", http.StatusInternalServerError)
			return
		}

		promoteAdmin(r, pool, cfg, user, "google")
		slog.Info("auth success", "method", "google", "user_id", user.ID, "email", user.Email)
		finishLogin(w, r, user)
	}
}

// githubCallback godoc
// @Summary     GitHub OAuth callback
// @Description Same shape as the Google callback (see finishLogin) - exchanges a code, finds or creates the user, issues a session.
// @Tags        auth
// @Accept      json
// @Produce     json
// @Param       body body object true "{\"code\": \"...\", \"redirect_uri\": \"...\"}"
// @Success     200 {object} object "{\"user\": accounts.User, \"token\": string}"
// @Failure     400 {string} string "invalid request body, redirect_uri not allowed, or GitHub account has no accessible email"
// @Failure     401 {string} string "code exchange or userinfo lookup failed"
// @Router      /auth/github/callback [post]
func githubCallback(cfg accounts.Config, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Code        string `json:"code"`
			RedirectURI string `json:"redirect_uri"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if !cfg.IsAllowedRedirect(body.RedirectURI) {
			slog.Warn("auth fail", "method", "github", "reason", "redirect_uri_not_allowed", "redirect_uri", body.RedirectURI)
			http.Error(w, "redirect_uri not allowed", http.StatusBadRequest)
			return
		}

		accessToken, err := cfg.ExchangeGitHubCode(r.Context(), body.Code, body.RedirectURI)
		if err != nil {
			slog.Warn("auth fail", "method", "github", "reason", "code_exchange_failed", "error", err)
			http.Error(w, "github code exchange failed", http.StatusUnauthorized)
			return
		}
		githubUser, err := cfg.GitHubUserInfo(r.Context(), accessToken)
		if err != nil {
			slog.Warn("auth fail", "method", "github", "reason", "userinfo_failed", "error", err)
			http.Error(w, "github userinfo failed", http.StatusUnauthorized)
			return
		}
		if githubUser.Email == "" {
			slog.Warn("auth fail", "method", "github", "reason", "no_email", "github_id", githubUser.ID)
			http.Error(w, "github account has no accessible email", http.StatusBadRequest)
			return
		}

		user, err := accounts.FindOrCreateGitHubUser(r.Context(), pool, githubUser)
		if err != nil {
			slog.Error("auth fail", "method", "github", "reason", "find_or_create_user_failed", "error", err)
			http.Error(w, "failed to find or create user", http.StatusInternalServerError)
			return
		}

		promoteAdmin(r, pool, cfg, user, "github")
		slog.Info("auth success", "method", "github", "user_id", user.ID, "email", user.Email)
		finishLogin(w, r, user)
	}
}

// promoteAdmin applies the ADMIN_EMAILS allowlist on an OAuth sign-in. Failure
// is logged, not fatal: the user is legitimately authenticated either way, and
// refusing the login over a failed role update would lock out the very admin
// the allowlist exists to let in. They'd land as role='user' and get admin on
// the next successful login.
func promoteAdmin(r *http.Request, pool *pgxpool.Pool, cfg accounts.Config, user *accounts.User, method string) {
	if _, err := accounts.PromoteIfAllowlisted(r.Context(), pool, cfg, user); err != nil {
		slog.Error("admin promotion failed", "method", method, "user_id", user.ID, "error", err)
	}
}

// finishLogin issues the session. The token goes into an httpOnly cookie and
// nothing else - the ttyd terminal iframe and its websocket can't attach an
// Authorization header at all (see internal/labs/proxy.go), so the cookie was
// always load-bearing; the frontend no longer keeps a second copy in
// localStorage. It's still returned in the body for API clients that want a
// Bearer token, which requireAuth also accepts.
func finishLogin(w http.ResponseWriter, r *http.Request, user *accounts.User) {
	// Single chokepoint for both login paths (Google, GitHub), so a blocked
	// account can't get a token from either. 403 rather than 401: the
	// credentials were fine, the account is disabled, and retrying won't help.
	if user.Blocked {
		slog.Info("auth rejected", "reason", "blocked", "user_id", user.ID)
		http.Error(w, "account blocked", http.StatusForbidden)
		return
	}

	token, err := auth.IssueLoginToken(user.ID, user.Email, sessionTTL)
	if err != nil {
		http.Error(w, "failed to issue session token", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, authCookie(r, token))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"user": user, "token": token})
}

// me godoc
// @Summary     Current user
// @Tags        auth
// @Produce     json
// @Success     200 {object} accounts.User
// @Failure     401 {string} string "unauthorized"
// @Security    BearerAuth
// @Router      /auth/me [get]
func me(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value(userIDKey).(string)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var user accounts.User
		err := pool.QueryRow(r.Context(),
			`SELECT id, email, username, oauth_provider, role, avatar IS NOT NULL FROM users WHERE id = $1`, userID,
		).Scan(&user.ID, &user.Email, &user.Username, &user.OAuthProvider, &user.Role, &user.HasAvatar)
		if err != nil {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(user)
	}
}

// logout godoc
// @Summary     Log out
// @Description Clears the auth cookie server-side; clearing the Bearer token from localStorage is the frontend's job (see lib/api.ts's logout()).
// @Tags        auth
// @Success     200
// @Router      /auth/logout [post]
func logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, clearAuthCookie(r))
	w.WriteHeader(http.StatusOK)
}

// bearerOrCookieToken reads the raw JWT from either an `Authorization:
// Bearer <token>` header (what the frontend sends after storing the token
// from finishLogin in localStorage) or the "token" cookie (what a plain
// browser navigation/iframe/websocket sends automatically, and can't be
// made to send a custom header at all). Header wins if both are present.
func bearerOrCookieToken(r *http.Request) (string, bool) {
	if auth := r.Header.Get("Authorization"); auth != "" {
		if token, ok := strings.CutPrefix(auth, "Bearer "); ok && token != "" {
			return token, true
		}
	}
	if cookie, err := r.Cookie("token"); err == nil && cookie.Value != "" {
		return cookie.Value, true
	}
	return "", false
}

// apiKeyFromRequest pulls an API key from either `X-API-Key: <key>` or
// `Authorization: Bearer <key>`.
//
// Accepting it on the Authorization header as well means an agent can use one
// convention for everything, and it's unambiguous because our keys carry a
// distinctive prefix (accounts.KeyPrefix): anything without it falls through to
// being treated as a JWT.
func apiKeyFromRequest(r *http.Request) (string, bool) {
	if k := strings.TrimSpace(r.Header.Get("X-API-Key")); k != "" {
		return k, true
	}
	if authz := r.Header.Get("Authorization"); authz != "" {
		if token, ok := strings.CutPrefix(authz, "Bearer "); ok {
			if strings.HasPrefix(token, accounts.KeyPrefix) {
				return token, true
			}
		}
	}
	return "", false
}

// requireAuth verifies the token (see bearerOrCookieToken), loads the caller's
// account, and injects both the user ID and the full user into the request
// context. 401s otherwise - used to gate anything that shouldn't be reachable
// without having signed in (session provisioning).
//
// The account is re-read from the DB on every request rather than trusted from
// the JWT's claims, for the same reason requireAdmin always did it: role
// changes and blocks must take effect immediately, not whenever an
// already-issued token happens to expire. A blocked user is rejected here, so
// blocking instantly kills API access for a token that's still cryptographically
// valid.
func requireAuth(pool *pgxpool.Pool, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// API key path. A key authenticates as its owner and carries no
		// permissions of its own - AuthenticateAPIKey re-reads the account, so
		// everything downstream (entitlements, requireAdmin) applies exactly as
		// it would to that user in a browser. Checked before the JWT path
		// because a key is unambiguous: it's prefix-tagged, so there's no
		// chance of misreading one as a JWT.
		if key, ok := apiKeyFromRequest(r); ok {
			u, scopes, err := accounts.AuthenticateAPIKey(r.Context(), pool, key)
			if err != nil {
				// AuthenticateAPIKey collapses unknown/expired/blocked into one
				// error on purpose - see ErrInvalidAPIKey.
				http.Error(w, "invalid api key", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), userIDKey, u.ID)
			ctx = context.WithValue(ctx, userKey, u)
			// Record that this is an API-key request and which scopes it carries,
			// so scope-gated handlers (e.g. driver upload) can enforce them. A
			// non-nil (possibly empty) slice marks "authenticated via key".
			ctx = context.WithValue(ctx, apiKeyScopesKey, scopesOrEmpty(scopes))
			next(w, r.WithContext(ctx))
			return
		}

		token, ok := bearerOrCookieToken(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		claims, err := auth.VerifyToken(token)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		// Confinement: an AI-scoped token is only ever valid on the /ai/* routes
		// (which check it via requireAIGrant). Refusing it here is what keeps a
		// leaked AI key off the entire user surface - profile, sessions, flag
		// submission - even though it carries a real user's sub. Deterministic, not
		// prompt-dependent.
		if auth.Scope(claims) == auth.ScopeAI {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		userID, ok := claims["sub"].(string)
		if !ok || userID == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		u, found, err := accounts.GetUser(r.Context(), pool, userID)
		if err != nil || !found {
			// Token references an account that no longer exists (deleted
			// user) or the DB is unreachable - either way the caller isn't
			// usefully authenticated.
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if u.Blocked {
			http.Error(w, "account blocked", http.StatusForbidden)
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, userID)
		ctx = context.WithValue(ctx, userKey, u)
		next(w, r.WithContext(ctx))
	}
}

// requireAdmin wraps requireAuth and additionally checks the caller's role.
// requireAuth has already loaded the account fresh from the DB, so this is
// just a field check - see its comment for why that's not read off the JWT.
func requireAdmin(pool *pgxpool.Pool, next http.HandlerFunc) http.HandlerFunc {
	return requireAuth(pool, func(w http.ResponseWriter, r *http.Request) {
		u := contextUser(r)
		if u == nil || !accounts.IsAdmin(u.Role) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	})
}

// contextUser returns the account requireAuth loaded, or nil if the handler
// wasn't wrapped in requireAuth.
func contextUser(r *http.Request) *accounts.User {
	u, _ := r.Context().Value(userKey).(*accounts.User)
	return u
}

// scopesOrEmpty makes sure a nil scope slice still stores a non-nil value in
// context, so apiKeyScopes can use "present" to mean "authenticated via key".
func scopesOrEmpty(scopes []string) []string {
	if scopes == nil {
		return []string{}
	}
	return scopes
}

// apiKeyScopes returns the scopes of the API key that authenticated this
// request, and whether the request was authenticated by an API key at all. A
// browser (cookie/JWT) session returns (nil, false) and is unrestricted; an
// API-key request returns (scopes, true) and is limited to what the key carries.
func apiKeyScopes(r *http.Request) ([]string, bool) {
	scopes, ok := r.Context().Value(apiKeyScopesKey).([]string)
	return scopes, ok
}

// optionalUser resolves the caller from an API key or JWT/cookie if a valid one
// is present, and returns nil otherwise - it never writes a response. It's for
// public endpoints that reveal a base view to everyone but personalize for a
// signed-in caller (the leaderboard: top ranks are public, the full board and
// "your rank" need an account). Mirrors requireAuth's resolution but degrades to
// anonymous instead of 401, and still refuses a blocked account.
func optionalUser(pool *pgxpool.Pool, r *http.Request) *accounts.User {
	if key, ok := apiKeyFromRequest(r); ok {
		u, _, err := accounts.AuthenticateAPIKey(r.Context(), pool, key)
		if err != nil {
			return nil
		}
		return u
	}
	token, ok := bearerOrCookieToken(r)
	if !ok {
		return nil
	}
	claims, err := auth.VerifyToken(token)
	if err != nil {
		return nil
	}
	// An AI-scoped token never authenticates a user anywhere but /ai/* (see
	// requireAuth for the full rationale) - degrade it to anonymous here too.
	if auth.Scope(claims) == auth.ScopeAI {
		return nil
	}
	userID, _ := claims["sub"].(string)
	if userID == "" {
		return nil
	}
	u, found, err := accounts.GetUser(r.Context(), pool, userID)
	if err != nil || !found || u.Blocked {
		return nil
	}
	return u
}
