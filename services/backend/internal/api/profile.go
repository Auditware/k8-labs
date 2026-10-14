package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"backend/internal/accounts"
)

// getProfile godoc
// @Summary     Get your profile
// @Description The signed-in user's own editable profile (website, bio, whether an avatar is set).
// @Tags        profile
// @Produce     json
// @Success     200 {object} accounts.Profile
// @Failure     401 {string} string "unauthorized"
// @Security    BearerAuth
// @Router      /profile [get]
func getProfile(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := contextUser(r)
		p, err := accounts.GetProfile(r.Context(), pool, u.ID)
		if err != nil {
			http.Error(w, "failed to load profile", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(p)
	}
}

// updateProfile godoc
// @Summary     Update your profile
// @Description Sets website and bio on your own account. Website must be an http(s) URL - it's rendered as a link, so other schemes are rejected.
// @Tags        profile
// @Accept      json
// @Produce     json
// @Param       body body object true "{\"website\": \"https://...\", \"bio\": \"...\"}"
// @Success     200 {object} accounts.Profile
// @Failure     400 {string} string "invalid website or bio too long"
// @Failure     401 {string} string "unauthorized"
// @Security    BearerAuth
// @Router      /profile [patch]
func updateProfile(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := contextUser(r)
		var body struct {
			Website  string `json:"website"`
			Bio      string `json:"bio"`
			X        string `json:"x"`
			LinkedIn string `json:"linkedin"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if err := accounts.UpdateProfile(r.Context(), pool, u.ID, body.Website, body.Bio, body.X, body.LinkedIn); err != nil {
			if errors.Is(err, accounts.ErrInvalidWebsite) || errors.Is(err, accounts.ErrBioTooLong) ||
				errors.Is(err, accounts.ErrInvalidX) || errors.Is(err, accounts.ErrInvalidLinkedIn) {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			http.Error(w, "failed to update profile", http.StatusInternalServerError)
			return
		}
		p, err := accounts.GetProfile(r.Context(), pool, u.ID)
		if err != nil {
			http.Error(w, "failed to load profile", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(p)
	}
}

// updateUsername godoc
// @Summary     Change your display name
// @Description Sets the handle shown on the leaderboard and used in your public /operators/{username} URL. 3-32 chars of letters, digits, _ or -; must be unique (case-insensitive).
// @Tags        profile
// @Accept      json
// @Produce     json
// @Param       body body object true "{\"username\": \"new_handle\"}"
// @Success     200 {object} object "{\"username\": \"new_handle\"}"
// @Failure     400 {string} string "invalid display name"
// @Failure     401 {string} string "unauthorized"
// @Failure     409 {string} string "display name already taken"
// @Security    BearerAuth
// @Router      /profile/username [patch]
func updateUsername(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := contextUser(r)
		var body struct {
			Username string `json:"username"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		username, err := accounts.UpdateUsername(r.Context(), pool, u.ID, body.Username)
		if err != nil {
			switch {
			case errors.Is(err, accounts.ErrInvalidUsername):
				http.Error(w, err.Error(), http.StatusBadRequest)
			case errors.Is(err, accounts.ErrUsernameTaken):
				http.Error(w, err.Error(), http.StatusConflict)
			default:
				http.Error(w, "failed to update display name", http.StatusInternalServerError)
			}
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"username": username})
	}
}

// uploadAvatar godoc
// @Summary     Upload your avatar
// @Description Raw image bytes as the request body (PNG/JPEG/GIF/WebP, max 1MB). The type is detected from the file's own magic bytes, not the Content-Type header. SVG is rejected - it can carry script and would be stored XSS served from this origin.
// @Tags        profile
// @Accept      octet-stream
// @Produce     json
// @Success     200 {object} map[string]string
// @Failure     400 {string} string "unsupported image type"
// @Failure     401 {string} string "unauthorized"
// @Failure     413 {string} string "image too large"
// @Security    BearerAuth
// @Router      /profile/avatar [post]
func uploadAvatar(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := contextUser(r)

		// Cap what we're willing to read into memory at all. Reading one byte
		// past the limit is how we tell "exactly at the limit" from "over it",
		// since MaxBytesReader errors only once it's exceeded.
		r.Body = http.MaxBytesReader(w, r.Body, accounts.MaxAvatarBytes+1)
		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "image too large", http.StatusRequestEntityTooLarge)
			return
		}

		if err := accounts.SetAvatar(r.Context(), pool, u.ID, data); err != nil {
			switch {
			case errors.Is(err, accounts.ErrAvatarTooLarge):
				http.Error(w, "image too large", http.StatusRequestEntityTooLarge)
			case errors.Is(err, accounts.ErrAvatarType):
				http.Error(w, "unsupported image type - use PNG, JPEG, GIF or WebP", http.StatusBadRequest)
			case errors.Is(err, accounts.ErrAvatarTooSmall):
				http.Error(w, fmt.Sprintf("image too small - use at least %dx%d pixels", accounts.MinAvatarPx, accounts.MinAvatarPx), http.StatusBadRequest)
			default:
				http.Error(w, "failed to store avatar", http.StatusInternalServerError)
			}
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// deleteAvatar godoc
// @Summary     Remove your avatar
// @Tags        profile
// @Produce     json
// @Success     200 {object} map[string]string
// @Failure     401 {string} string "unauthorized"
// @Security    BearerAuth
// @Router      /profile/avatar [delete]
func deleteAvatar(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := contextUser(r)
		if err := accounts.DeleteAvatar(r.Context(), pool, u.ID); err != nil {
			http.Error(w, "failed to remove avatar", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// serveAvatar returns a user's avatar image.
//
// Readable by any signed-in user, not just its owner: avatars are shown in the
// admin Users list, so restricting it to self would break that. It exposes only
// a picture the user chose to publish on their own profile.
//
// Deliberately unannotated for swaggo - it returns image bytes, not JSON.
func serveAvatar(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, contentType, err := accounts.GetAvatar(r.Context(), pool, r.PathValue("id"))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "failed to load avatar", http.StatusInternalServerError)
			return
		}

		// The content type comes from our own allowlist (set by sniffing the
		// bytes at upload), never from the uploader. nosniff stops the browser
		// second-guessing it and treating the response as something
		// executable; the CSP is belt-and-braces for the same reason. Private
		// caching only - this is behind auth and shouldn't sit in a shared
		// proxy.
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
		w.Header().Set("Cache-Control", "private, max-age=300")
		w.Write(data)
	}
}

// listAPIKeys godoc
// @Summary     List your API keys
// @Description Metadata only. Key secrets are stored hashed and are unrecoverable after creation.
// @Tags        api-keys
// @Produce     json
// @Success     200 {array} accounts.APIKey
// @Failure     401 {string} string "unauthorized"
// @Security    BearerAuth
// @Router      /apikeys [get]
func listAPIKeys(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := contextUser(r)
		keys, err := accounts.ListAPIKeys(r.Context(), pool, u.ID)
		if err != nil {
			http.Error(w, "failed to list api keys", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(keys)
	}
}

// createAPIKey godoc
// @Summary     Create an API key
// @Description Returns the key once, in the "key" field - it is not stored in plaintext and cannot be retrieved again. The key authenticates as you: same labs, same role, and it stops working the moment your access is revoked or your account is blocked. expiresInDays omitted defaults to 30; 0 means never expires.
// @Tags        api-keys
// @Accept      json
// @Produce     json
// @Param       body body object true "{\"name\": \"my agent\", \"expiresInDays\": 30}"
// @Success     200 {object} object "{\"key\": \"k8labs_...\", \"apiKey\": accounts.APIKey}"
// @Failure     401 {string} string "unauthorized"
// @Security    BearerAuth
// @Router      /apikeys [post]
func createAPIKey(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := contextUser(r)
		var body struct {
			Name string `json:"name"`
			// Pointer so "field absent" (use the default) is distinguishable
			// from an explicit 0 (never expires).
			ExpiresInDays *int `json:"expiresInDays"`
			// Scopes optionally restrict/authorize the key for sensitive
			// capabilities. Each must be a known scope, and a scope the caller
			// isn't entitled to is refused (see below).
			Scopes []string `json:"scopes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		// Authorize each requested scope. drivers:write may only go on a key whose
		// owner has research-lab access - so an unentitled user cannot mint a key
		// that would out-privilege them, and the ability to create such a key is
		// itself the "granted with research access" permission.
		for _, s := range body.Scopes {
			if !accounts.ValidAPIKeyScope(s) {
				http.Error(w, "unknown scope: "+s, http.StatusBadRequest)
				return
			}
			if s == accounts.ScopeDriversWrite {
				ok, err := hasResearchAccess(r, pool, u)
				if err != nil {
					http.Error(w, "failed to read entitlements", http.StatusInternalServerError)
					return
				}
				if !ok {
					http.Error(w, "forbidden - the drivers:write scope needs research lab access", http.StatusForbidden)
					return
				}
			}
		}

		expiresAt := timePtr(time.Now().Add(accounts.DefaultKeyTTL))
		if body.ExpiresInDays != nil {
			switch {
			case *body.ExpiresInDays == 0:
				expiresAt = nil // explicit "never expires"
			case *body.ExpiresInDays < 0:
				http.Error(w, "expiresInDays cannot be negative", http.StatusBadRequest)
				return
			default:
				expiresAt = timePtr(time.Now().AddDate(0, 0, *body.ExpiresInDays))
			}
		}

		key, plaintext, err := accounts.CreateAPIKey(r.Context(), pool, u.ID, body.Name, expiresAt, body.Scopes)
		if err != nil {
			http.Error(w, "failed to create api key", http.StatusInternalServerError)
			return
		}
		// The only time the plaintext ever leaves this process.
		json.NewEncoder(w).Encode(map[string]any{"key": plaintext, "apiKey": key})
	}
}

// deleteAPIKey godoc
// @Summary     Revoke an API key
// @Description Revokes one of your own keys. 404 if no such key belongs to you - the same answer whether it doesn't exist or belongs to someone else.
// @Tags        api-keys
// @Produce     json
// @Param       id path string true "key ID"
// @Success     200 {object} map[string]string
// @Failure     401 {string} string "unauthorized"
// @Failure     404 {string} string "no such key"
// @Security    BearerAuth
// @Router      /apikeys/{id} [delete]
func deleteAPIKey(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := contextUser(r)
		// Scoped to the caller in the query itself, so a guessed ID belonging
		// to someone else deletes nothing - and must not then report success.
		deleted, err := accounts.DeleteAPIKey(r.Context(), pool, u.ID, r.PathValue("id"))
		if err != nil {
			http.Error(w, "failed to revoke api key", http.StatusInternalServerError)
			return
		}
		if !deleted {
			http.Error(w, "no such key", http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func timePtr(t time.Time) *time.Time { return &t }
