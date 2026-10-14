package drivers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Lifecycle values. A driver is pending until an admin acts; approved drivers
// are usable, denied ones are kept (with a reason) so the same bad .ko is not
// re-litigated on every re-upload.
const (
	StatusPending  = "pending"
	StatusApproved = "approved"
	StatusDenied   = "denied"

	// VisibilityPrivate: usable only by its uploader. VisibilityCatalog: in the
	// shared catalog, usable by every research user. Only an admin promotes to
	// catalog, and only on approve.
	VisibilityPrivate = "private"
	VisibilityCatalog = "catalog"
)

// Driver is one catalog row. KoBytes is populated only by GetBytes /
// ResolveForSession - the list/queue queries deliberately leave it nil so a
// listing never drags every module's blob through memory.
type Driver struct {
	ID          string     `json:"id"`
	SHA256      string     `json:"sha256"`
	Filename    string     `json:"filename"`
	SizeBytes   int        `json:"sizeBytes"`
	ModName     string     `json:"modName"`
	Vermagic    string     `json:"vermagic"`
	License     string     `json:"license"`
	Depends     string     `json:"depends"`
	Description string     `json:"description"`
	IsSigned    bool       `json:"isSigned"`
	Signer      string     `json:"signer,omitempty"`
	Status      string     `json:"status"`
	Visibility  string     `json:"visibility"`
	UploadedBy  string     `json:"uploadedBy"`
	UploadedAt  time.Time  `json:"uploadedAt"`
	VettedBy    *string    `json:"vettedBy,omitempty"`
	VettedAt    *time.Time `json:"vettedAt,omitempty"`
	VetReason   string     `json:"vetReason,omitempty"`
	KoBytes     []byte     `json:"-"`
}

// ErrNotFound is returned when a driver id does not exist. ErrDuplicate is
// returned by Create when the same content (by sha256) is already on file; the
// existing row rides along so the caller can tell the user its current status
// (notably a re-upload of an already-denied module).
var (
	ErrNotFound  = errors.New("driver not found")
	ErrDuplicate = errors.New("driver already uploaded")
)

// Per-user upload limits. The .ko bytes live in Postgres bytea (up to MaxKoBytes
// each), so without a bound one research user could fill the store. Two limits,
// both enforced by EnforceUploadLimits and both exempted for admins:
//
//   - MaxPendingPerUser caps how many of a user's own uploads may sit unvetted
//     at once. It counts only pending rows, so it bounds the storage and the
//     vetting-queue backlog a user can create UNILATERALLY, self-heals as an
//     admin works the queue (approve/deny drops the count), and never penalises
//     a user whose modules got approved (approved storage is the admin's call).
//   - MaxUploadsPerWindow caps uploads in a trailing window, throttling a flood
//     that would otherwise add rows as fast as the client can POST.
const (
	MaxPendingPerUser   = 20
	MaxUploadsPerWindow = 15
	uploadRateWindow    = time.Hour
)

// ErrQuotaExceeded (too many pending uploads) and RateLimitedError (too many
// uploads too fast) are the two rejections EnforceUploadLimits returns. The rate
// error carries how long until the caller may retry, for a Retry-After header.
var ErrQuotaExceeded = errors.New("too many pending driver uploads")

// RateLimitedError signals the trailing-window upload rate was exceeded.
type RateLimitedError struct{ RetryAfter time.Duration }

func (e *RateLimitedError) Error() string { return "driver upload rate limit exceeded" }

// EnforceUploadLimits rejects an upload that would breach the caller's pending
// quota or upload rate. Returns nil to allow, ErrQuotaExceeded, or a
// *RateLimitedError. Both checks read only stored rows (a rejected parse or a
// deduped re-upload never creates one, so neither counts). The caller exempts
// admins and checks this before reading the request body, so a throttled upload
// does not stream megabytes first.
func EnforceUploadLimits(ctx context.Context, pool *pgxpool.Pool, userID string) error {
	// Rate first: uploads inside the trailing window. min(uploaded_at) gives an
	// exact "retry when the oldest one ages out" rather than a guessed constant.
	cutoff := time.Now().Add(-uploadRateWindow)
	var recent int
	var oldest *time.Time
	err := pool.QueryRow(ctx,
		`SELECT count(*), min(uploaded_at) FROM research_drivers WHERE uploaded_by=$1 AND uploaded_at > $2`,
		userID, cutoff).Scan(&recent, &oldest)
	if err != nil {
		return err
	}
	if recent >= MaxUploadsPerWindow && oldest != nil {
		retry := time.Until(oldest.Add(uploadRateWindow))
		if retry < 0 {
			retry = 0
		}
		return &RateLimitedError{RetryAfter: retry}
	}

	// Quota: pending (unvetted) rows the user currently owns.
	var pending int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM research_drivers WHERE uploaded_by=$1 AND status=$2`,
		userID, StatusPending).Scan(&pending); err != nil {
		return err
	}
	if pending >= MaxPendingPerUser {
		return ErrQuotaExceeded
	}
	return nil
}

// listCols is every column except the blob - the shape every listing scans.
const listCols = `id, sha256, filename, size_bytes, mod_name, vermagic, license, depends,
	description, is_signed, signer, status, visibility, uploaded_by, uploaded_at,
	vetted_by, vetted_at, vet_reason`

func scanDriver(row pgx.Row) (Driver, error) {
	var d Driver
	err := row.Scan(&d.ID, &d.SHA256, &d.Filename, &d.SizeBytes, &d.ModName, &d.Vermagic,
		&d.License, &d.Depends, &d.Description, &d.IsSigned, &d.Signer, &d.Status,
		&d.Visibility, &d.UploadedBy, &d.UploadedAt, &d.VettedBy, &d.VettedAt, &d.VetReason)
	return d, err
}

func scanDrivers(rows pgx.Rows) ([]Driver, error) {
	defer rows.Close()
	out := []Driver{}
	for rows.Next() {
		d, err := scanDriver(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// Create inserts a freshly-parsed upload as pending/private. If the same content
// is already stored it returns that row with ErrDuplicate rather than a second
// copy - the sha256 is the dedup key.
func Create(ctx context.Context, pool *pgxpool.Pool, uploaderID, filename string, size int, data []byte, p ParsedKo) (Driver, error) {
	if existing, err := getBySHA(ctx, pool, p.SHA256); err == nil {
		return existing, ErrDuplicate
	} else if !errors.Is(err, ErrNotFound) {
		return Driver{}, err
	}
	id, err := randomID()
	if err != nil {
		return Driver{}, err
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO research_drivers
			(id, sha256, filename, size_bytes, ko_bytes, mod_name, vermagic, license,
			 depends, description, is_signed, signer, status, visibility, uploaded_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		id, p.SHA256, filename, size, data, p.ModName, p.Vermagic, p.License,
		p.Depends, p.Description, p.IsSigned, p.Signer, StatusPending, VisibilityPrivate, uploaderID)
	if err != nil {
		return Driver{}, err
	}
	return getByID(ctx, pool, id)
}

func getBySHA(ctx context.Context, pool *pgxpool.Pool, sha string) (Driver, error) {
	d, err := scanDriver(pool.QueryRow(ctx, `SELECT `+listCols+` FROM research_drivers WHERE sha256=$1`, sha))
	if errors.Is(err, pgx.ErrNoRows) {
		return Driver{}, ErrNotFound
	}
	return d, err
}

func getByID(ctx context.Context, pool *pgxpool.Pool, id string) (Driver, error) {
	d, err := scanDriver(pool.QueryRow(ctx, `SELECT `+listCols+` FROM research_drivers WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Driver{}, ErrNotFound
	}
	return d, err
}

// Get returns one driver's metadata (no blob).
func Get(ctx context.Context, pool *pgxpool.Pool, id string) (Driver, error) {
	return getByID(ctx, pool, id)
}

// ListForUser returns the catalog a research user may pick from: every approved
// shared-catalog driver plus the user's own uploads (any status), newest first.
// Blobs are omitted. This is what the in-lab picker renders.
func ListForUser(ctx context.Context, pool *pgxpool.Pool, userID string) ([]Driver, error) {
	rows, err := pool.Query(ctx, `SELECT `+listCols+` FROM research_drivers
		WHERE (status=$1 AND visibility=$2) OR uploaded_by=$3
		ORDER BY uploaded_at DESC`, StatusApproved, VisibilityCatalog, userID)
	if err != nil {
		return nil, err
	}
	return scanDrivers(rows)
}

// ListByStatus returns every driver in a lifecycle state, newest first - the
// admin vetting queue (status=pending) and its approved/denied views.
func ListByStatus(ctx context.Context, pool *pgxpool.Pool, status string) ([]Driver, error) {
	rows, err := pool.Query(ctx, `SELECT `+listCols+` FROM research_drivers
		WHERE status=$1 ORDER BY uploaded_at DESC`, status)
	if err != nil {
		return nil, err
	}
	return scanDrivers(rows)
}

// GetBytes returns a driver's raw .ko and its filename, for the admin download
// and for session injection.
func GetBytes(ctx context.Context, pool *pgxpool.Pool, id string) ([]byte, string, error) {
	var data []byte
	var filename string
	err := pool.QueryRow(ctx, `SELECT ko_bytes, filename FROM research_drivers WHERE id=$1`, id).Scan(&data, &filename)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	return data, filename, err
}

// Decision is an admin's allow/deny. Approve can promote to the shared catalog
// (visibility=catalog) or keep it private; deny keeps the row with a reason.
type Decision struct {
	Status     string // StatusApproved or StatusDenied
	Visibility string // on approve: VisibilityCatalog or VisibilityPrivate
	Reason     string
	VettedBy   string // admin user id
}

// SetDecision records an admin's vetting decision.
func SetDecision(ctx context.Context, pool *pgxpool.Pool, id string, d Decision) error {
	if d.Status != StatusApproved && d.Status != StatusDenied {
		return fmt.Errorf("invalid decision status %q", d.Status)
	}
	vis := VisibilityPrivate
	if d.Status == StatusApproved && d.Visibility == VisibilityCatalog {
		vis = VisibilityCatalog
	}
	tag, err := pool.Exec(ctx, `UPDATE research_drivers
		SET status=$1, visibility=$2, vet_reason=$3, vetted_by=$4, vetted_at=now()
		WHERE id=$5`, d.Status, vis, d.Reason, d.VettedBy, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// keyRe is what a Kubernetes Secret data key permits.
var keyRe = regexp.MustCompile(`[^-._a-zA-Z0-9]`)

// indexEntry is one line of the /opt/drivers/index.json manifest the in-guest
// research-driver helper reads to list drivers and show their vermagic.
type indexEntry struct {
	File        string `json:"file"`
	Name        string `json:"name"`
	Vermagic    string `json:"vermagic"`
	Description string `json:"description,omitempty"`
}

// ResolveForSession turns a set of driver ids chosen at session-create into the
// Secret data map mounted at /opt/drivers. It enforces entitlement: an id is
// included only if it is approved AND either in the shared catalog or owned by
// this user - so a user cannot mount a pending, denied, or someone else's
// private module by guessing an id. Unknown or unentitled ids are silently
// skipped (the picker only ever offers legitimate ones). Returns nil when
// nothing resolves, so a no-driver session mounts no Secret.
//
// Keys are "<sanitized-name>-<short-hash>.ko" so they are always valid Secret
// keys and never collide. An index.json rides alongside for the helper.
func ResolveForSession(ctx context.Context, pool *pgxpool.Pool, userID string, ids []string) (map[string][]byte, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := pool.Query(ctx, `SELECT id, mod_name, filename, vermagic, description, ko_bytes
		FROM research_drivers
		WHERE id = ANY($1) AND status=$2 AND (visibility=$3 OR uploaded_by=$4)`,
		ids, StatusApproved, VisibilityCatalog, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	data := map[string][]byte{}
	index := []indexEntry{}
	for rows.Next() {
		var id, modName, filename, vermagic, description string
		var ko []byte
		if err := rows.Scan(&id, &modName, &filename, &vermagic, &description, &ko); err != nil {
			return nil, err
		}
		key := secretKey(modName, filename, id)
		data[key] = ko
		index = append(index, indexEntry{File: key, Name: displayName(modName, filename), Vermagic: vermagic, Description: description})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	idxJSON, err := json.Marshal(index)
	if err != nil {
		return nil, err
	}
	data["index.json"] = idxJSON
	return data, nil
}

// secretKey builds a valid, unique, .ko-suffixed Secret key from a module's
// name (falling back to its filename base) plus a short slice of its id.
func secretKey(modName, filename, id string) string {
	base := displayName(modName, filename)
	base = keyRe.ReplaceAllString(base, "_")
	if base == "" {
		base = "driver"
	}
	short := id
	if len(short) > 6 {
		short = short[:6]
	}
	return fmt.Sprintf("%s-%s.ko", base, short)
}

// displayName prefers the module's own name, else the filename with any .ko
// stripped.
func displayName(modName, filename string) string {
	if modName != "" {
		return modName
	}
	return strings.TrimSuffix(filename, ".ko")
}

// randomID mirrors accounts.randomID: 8 crypto-random bytes, hex-encoded.
func randomID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
