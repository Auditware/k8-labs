package accounts

import (
	"bytes"
	"context"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/url"
	"strings"

	_ "golang.org/x/image/webp"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MaxAvatarBytes caps what lands in the users table. Avatars are stored as
// bytea (no object storage in this stack - see db/schema.go), so this is the
// only thing standing between the table and unbounded growth.
//
// 1MB, not the old 256KB: a photo straight off a phone clears 256KB routinely,
// and the failure was a rejected upload rather than anything the user could
// act on. The row cost is bounded by the user count either way.
const MaxAvatarBytes = 1024 * 1024

// MinAvatarPx is the smallest width/height an avatar may have. Magic bytes
// alone let a 1x1 transparent pixel through, which stores hasAvatar=true and
// then renders as an empty circle everywhere instead of the initial fallback.
// The frontend <Avatar> applies the same floor to rows stored before this check.
const MinAvatarPx = 16

// MaxBioBytes / MaxWebsiteBytes bound the free-text fields. Not a security
// control on their own - the output escaping is - just sane limits so a row
// can't be megabytes of prose.
const (
	MaxBioBytes     = 1000
	MaxWebsiteBytes = 200
	MaxSocialBytes  = 200
)

var (
	ErrAvatarTooLarge  = errors.New("avatar exceeds the size limit")
	ErrAvatarType      = errors.New("unsupported image type")
	ErrAvatarTooSmall  = errors.New("avatar dimensions are too small")
	ErrInvalidWebsite  = errors.New("website must be an http(s) URL")
	ErrBioTooLong      = errors.New("bio is too long")
	ErrInvalidX        = errors.New("X link must be an https x.com/twitter.com URL")
	ErrInvalidLinkedIn = errors.New("LinkedIn link must be an https linkedin.com URL")
)

// allowedAvatarTypes is an allowlist, and SVG is deliberately absent.
//
// An SVG is a document, not just pixels: it can carry <script>, external
// entity references, and event handlers, so serving a user-supplied one from
// our own origin would be stored XSS against every viewer. The raster formats
// below can't execute. This is also why the avatar is served with a fixed
// Content-Type from this list plus X-Content-Type-Options: nosniff rather than
// echoing back whatever the uploader claimed - see the avatar handler.
var allowedAvatarTypes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/gif":  true,
	"image/webp": true,
}

// sniffAvatarType identifies the image from its magic bytes and returns the
// canonical content type. The uploader's declared Content-Type is not trusted -
// it's attacker-controlled, and believing it is how an "image/png" that's
// actually an SVG or an HTML document gets served back from our origin.
func sniffAvatarType(data []byte) (string, error) {
	switch {
	case len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n":
		return "image/png", nil
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return "image/jpeg", nil
	case len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a"):
		return "image/gif", nil
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return "image/webp", nil
	}
	return "", ErrAvatarType
}

// checkAvatarDimensions parses the image header (no full decode) and rejects
// anything smaller than MinAvatarPx on either side. A header that won't parse is
// a type error: the magic bytes matched but the rest isn't a real image.
func checkAvatarDimensions(data []byte) error {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return ErrAvatarType
	}
	if cfg.Width < MinAvatarPx || cfg.Height < MinAvatarPx {
		return ErrAvatarTooSmall
	}
	return nil
}

// Profile is the self-editable part of an account.
type Profile struct {
	Website   string `json:"website"`
	Bio       string `json:"bio"`
	X         string `json:"x"`        // https x.com / twitter.com profile URL, or ""
	LinkedIn  string `json:"linkedin"` // https linkedin.com profile URL, or ""
	HasAvatar bool   `json:"hasAvatar"`
}

// GetProfile reads a user's profile fields. The avatar bytes are deliberately
// not selected here - they're served by their own endpoint so a profile fetch
// doesn't drag the image bytes through every JSON response.
func GetProfile(ctx context.Context, pool *pgxpool.Pool, userID string) (*Profile, error) {
	var p Profile
	err := pool.QueryRow(ctx,
		`SELECT website, bio, x_url, linkedin_url, avatar IS NOT NULL FROM users WHERE id = $1`, userID,
	).Scan(&p.Website, &p.Bio, &p.X, &p.LinkedIn, &p.HasAvatar)
	if err == pgx.ErrNoRows {
		return nil, pgx.ErrNoRows
	}
	return &p, err
}

// UpdateProfile writes the free-text profile fields.
func UpdateProfile(ctx context.Context, pool *pgxpool.Pool, userID, website, bio, x, linkedin string) error {
	website = strings.TrimSpace(website)
	bio = strings.TrimSpace(bio)
	x = strings.TrimSpace(x)
	linkedin = strings.TrimSpace(linkedin)

	if len(bio) > MaxBioBytes {
		return ErrBioTooLong
	}
	if err := validateWebsite(website); err != nil {
		return err
	}
	if err := validateSocial(x, []string{"x.com", "twitter.com"}, ErrInvalidX); err != nil {
		return err
	}
	if err := validateSocial(linkedin, []string{"linkedin.com"}, ErrInvalidLinkedIn); err != nil {
		return err
	}

	_, err := pool.Exec(ctx,
		`UPDATE users SET website = $1, bio = $2, x_url = $3, linkedin_url = $4 WHERE id = $5`,
		website, bio, x, linkedin, userID)
	return err
}

// validateSocial restricts a social link to an https URL on one of the allowed
// hosts (bare or www.). Same security purpose as validateWebsite - the value is
// rendered as a link, so javascript:/data: and off-domain links are rejected -
// plus a host allowlist so the "X" / "LinkedIn" icons genuinely point where they
// claim. Empty is fine; the fields are optional.
func validateSocial(link string, hosts []string, invalid error) error {
	if link == "" {
		return nil
	}
	if len(link) > MaxSocialBytes {
		return invalid
	}
	u, err := url.Parse(link)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return invalid
	}
	host := strings.ToLower(u.Host)
	for _, h := range hosts {
		if host == h || host == "www."+h {
			return nil
		}
	}
	return invalid
}

// validateWebsite restricts the field to http(s) URLs.
//
// This is a security check, not tidiness: the value is rendered as a link, and
// permitting javascript: or data: would turn a profile into a stored XSS vector
// for anyone who clicks it. An empty value is fine - the field is optional.
func validateWebsite(website string) error {
	if website == "" {
		return nil
	}
	if len(website) > MaxWebsiteBytes {
		return ErrInvalidWebsite
	}
	u, err := url.Parse(website)
	if err != nil {
		return ErrInvalidWebsite
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ErrInvalidWebsite
	}
	if u.Host == "" {
		return ErrInvalidWebsite
	}
	return nil
}

// SetAvatar validates and stores an avatar. The content type is derived from
// the bytes themselves (see sniffAvatarType), never from what the client said.
func SetAvatar(ctx context.Context, pool *pgxpool.Pool, userID string, data []byte) error {
	if len(data) > MaxAvatarBytes {
		return ErrAvatarTooLarge
	}
	if len(data) == 0 {
		return ErrAvatarType
	}
	contentType, err := sniffAvatarType(data)
	if err != nil {
		return err
	}
	if !allowedAvatarTypes[contentType] {
		return ErrAvatarType
	}
	if err := checkAvatarDimensions(data); err != nil {
		return err
	}
	_, err = pool.Exec(ctx,
		`UPDATE users SET avatar = $1, avatar_content_type = $2 WHERE id = $3`,
		data, contentType, userID)
	return err
}

// GetAvatar returns the stored image and its canonical content type.
func GetAvatar(ctx context.Context, pool *pgxpool.Pool, userID string) ([]byte, string, error) {
	var (
		data        []byte
		contentType *string
	)
	err := pool.QueryRow(ctx,
		`SELECT avatar, avatar_content_type FROM users WHERE id = $1`, userID,
	).Scan(&data, &contentType)
	if err != nil {
		return nil, "", err
	}
	if len(data) == 0 || contentType == nil {
		return nil, "", pgx.ErrNoRows
	}
	// Re-check against the allowlist on the way out as well as in. A row could
	// predate a tightening of the list, and serving from our own origin is
	// exactly where being wrong is expensive.
	if !allowedAvatarTypes[*contentType] {
		return nil, "", ErrAvatarType
	}
	return data, *contentType, nil
}

// DeleteAvatar clears a user's avatar.
func DeleteAvatar(ctx context.Context, pool *pgxpool.Pool, userID string) error {
	_, err := pool.Exec(ctx,
		`UPDATE users SET avatar = NULL, avatar_content_type = NULL WHERE id = $1`, userID)
	return err
}
