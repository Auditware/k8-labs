package accounts

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"backend/internal/labs"
)

// UsageStore records what sessions cost and answers the quota question. Mirrors
// TimingStore: the labs package holds no database dependency and receives this
// through an interface.
type UsageStore struct {
	pool *pgxpool.Pool
}

func NewUsageStore(pool *pgxpool.Pool) *UsageStore {
	return &UsageStore{pool: pool}
}

// RecordSession writes one finished session's cost.
//
// Never returns an error to the caller's control flow by design - it is invoked
// from the reaper and from Delete, where the session is already gone and there
// is nothing to roll back. A failed write loses one row of accounting; failing
// the teardown instead would leak a runner pod and its whole microVM, which is
// far worse. Logged loudly so the loss is visible.
func (s *UsageStore) RecordSession(rec labs.UsageRecord) {
	if s == nil || s.pool == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := s.pool.Exec(ctx, `
		INSERT INTO lab_usage (session_id, user_id, module_slug, started_at, ended_at, gib_minutes, ended_reason)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (session_id) DO NOTHING`,
		rec.SessionID, rec.UserID, rec.ModuleSlug, rec.StartedAt, rec.EndedAt,
		rec.GiBMinutes, rec.Reason)
	if err != nil {
		// ON CONFLICT DO NOTHING covers the benign double-record (reaper and an
		// explicit stop racing). Anything reaching here is a real failure.
		slog.Error("usage: record failed", "session", rec.SessionID, "user", rec.UserID, "error", err)
	}
}

// UsedSince totals a user's consumption over the trailing window, in
// GiB-minutes.
//
// On a database error it returns 0, which admits the session. That is the
// deliberate direction to fail: a database blip must not lock every student out
// of every lab. The capacity budget and the per-user concurrency cap are both
// still in force and neither depends on Postgres, so the OOM guarantee holds
// even while this is degraded - only the fairness limit relaxes.
func (s *UsageStore) UsedSince(userID string, window time.Duration) float64 {
	if s == nil || s.pool == nil {
		return 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var total float64
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(gib_minutes), 0) FROM lab_usage
		WHERE user_id = $1 AND ended_at > $2`,
		userID, time.Now().Add(-window)).Scan(&total)
	if err != nil {
		slog.Error("usage: quota lookup failed, admitting", "user", userID, "error", err)
		return 0
	}
	return total
}

// QuotaOverride returns a user's per-user daily quota override (in GiB-minutes)
// and whether one is set. Implements the extra half of labs.QuotaSource so the
// admission gate can cap each user by their own allowance rather than only the
// global default. A missing user or a NULL column reports "no override", and the
// manager then falls back to the global config value.
//
// A database error reports "no override" (fall back to global) rather than
// blocking - same fail-open direction as UsedSince, so a Postgres blip relaxes
// the per-user quota rather than locking people out.
func (s *UsageStore) QuotaOverride(userID string) (float64, bool) {
	if s == nil || s.pool == nil || userID == "" {
		return 0, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var q *float64
	err := s.pool.QueryRow(ctx, `SELECT daily_quota_gib_minutes FROM users WHERE id = $1`, userID).Scan(&q)
	if err != nil || q == nil {
		return 0, false
	}
	return *q, true
}

// UserUsage is the per-user rollup the admin User Management page shows.
// Components stay separate here too - the UI decides how to present them, and a
// blended figure computed in SQL would be just as unretunable as a blended
// column.
type UserUsage struct {
	UserID string `json:"userId"`
	// Trailing-window figures: what the quota actually acts on.
	WindowGiBMinutes float64 `json:"windowGibMinutes"`
	WindowSessions   int     `json:"windowSessions"`
	// Lifetime figures: who is heavy overall, independent of the window.
	TotalGiBMinutes  float64    `json:"totalGibMinutes"`
	TotalSessions    int        `json:"totalSessions"`
	TotalCPUSeconds  float64    `json:"totalCpuSeconds"`
	TotalEgressBytes int64      `json:"totalEgressBytes"`
	LastSessionAt    *time.Time `json:"lastSessionAt,omitempty"`
}

// ListUserUsage returns the rollup for every user with recorded usage, keyed by
// user id. One query rather than per-user lookups so the admin page stays a
// single round trip as the user count grows.
func ListUserUsage(ctx context.Context, pool *pgxpool.Pool) (map[string]UserUsage, error) {
	rows, err := pool.Query(ctx, `
		SELECT user_id,
		       COALESCE(SUM(gib_minutes) FILTER (WHERE ended_at > $1), 0),
		       COUNT(*)  FILTER (WHERE ended_at > $1),
		       COALESCE(SUM(gib_minutes), 0),
		       COUNT(*),
		       COALESCE(SUM(cpu_seconds), 0),
		       COALESCE(SUM(egress_bytes), 0),
		       MAX(ended_at)
		FROM lab_usage
		GROUP BY user_id`, time.Now().Add(-labs.QuotaWindow))
	if err != nil {
		return nil, fmt.Errorf("list user usage: %w", err)
	}
	defer rows.Close()

	out := map[string]UserUsage{}
	for rows.Next() {
		var u UserUsage
		if err := rows.Scan(&u.UserID, &u.WindowGiBMinutes, &u.WindowSessions,
			&u.TotalGiBMinutes, &u.TotalSessions, &u.TotalCPUSeconds,
			&u.TotalEgressBytes, &u.LastSessionAt); err != nil {
			return nil, err
		}
		out[u.UserID] = u
	}
	return out, rows.Err()
}
