package db

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// seedEmailDomain picks the throwaway domain for seeded accounts by environment.
// In prod CORS_ORIGIN is the real site origin; locally it's unset or a localhost
// URL. Seeded accounts never receive mail - the domain only marks them as
// synthetic and keeps their (unique) emails from ever colliding with a real one.
func seedEmailDomain() string {
	origin := os.Getenv("CORS_ORIGIN")
	if origin != "" && !strings.Contains(origin, "localhost") && !strings.Contains(origin, "127.0.0.1") {
		return "artificialseed.prod"
	}
	return "artificialseed.local"
}

type seedUser struct {
	id              string
	username        string
	joinedMonthsAgo int
	// module slug -> solve seconds, one entry per lab this operator has cleared.
	// The number of entries is their labs-cleared; the minimum is their best
	// solve time. Counts are 3/2/1 so labs-cleared alone ranks them 1/2/3.
	solves     map[string]int
	streakDays int
}

// The three showcase operators. They exist as real but non-login accounts (no
// google/github id, no password, provider "seed") so the leaderboard and the
// landing both render genuine, queryable data rather than hardcoded fixtures -
// nothing in the frontend hardcodes a handle or a time, it all reads back from
// these rows. The landing deck shows exactly the top showcaseCount (3) by rank,
// which is why three is the count: enough to fill the deck, and no more synthetic
// operators than that on the public board (an earlier version seeded a longer
// fake roster purely to populate the leaderboard's gated tail - dropped as more
// invented data than the public surface should carry).
//
// Ordering is the board's own: most labs first, then fastest solve, then streak.
// They lead 13/10/8 labs cleared and keep their card anchors - r00tkitten's 252s
// (4:12) best and 14-day streak, nullbyte 408s +9, capsh_kelly 450s +6 - now that
// there are 13 labs to clear, they simply clear more of them. Each operator's map
// minimum is their best time; harder labs (higher curriculum Order) carry the
// longer solves. This slice is only planted into a fresh database (see
// SeedDefaultUsers); on an established one the live rows are left as the admins
// have curated them, deletions included.
var seedUsers = []seedUser{
	{id: "seed-r00tkitten", username: "r00tkitten", joinedMonthsAgo: 4, streakDays: 14,
		solves: map[string]int{
			"ns-escape": 252, "suid-privesc": 300, "ebpf-detect": 288, "sa-impersonate": 276,
			"pod-exec": 312, "create-pods": 348, "sidecar-inject": 402, "rolebinding-escalate": 384,
			"hostpath-escape": 294, "privileged-escape": 330, "node-token-harvest": 444,
			"static-pod-persist": 498, "malicious-webhook": 612}},
	{id: "seed-nullbyte", username: "nullbyte", joinedMonthsAgo: 6, streakDays: 9,
		solves: map[string]int{
			"ns-escape": 408, "suid-privesc": 470, "ebpf-detect": 462, "sa-impersonate": 444,
			"pod-exec": 456, "create-pods": 492, "sidecar-inject": 540, "rolebinding-escalate": 522,
			"hostpath-escape": 468, "privileged-escape": 510}},
	{id: "seed-capsh_kelly", username: "capsh_kelly", joinedMonthsAgo: 3, streakDays: 6,
		solves: map[string]int{
			"ns-escape": 450, "suid-privesc": 486, "ebpf-detect": 498, "sa-impersonate": 468,
			"pod-exec": 480, "create-pods": 522, "sidecar-inject": 564, "rolebinding-escalate": 540}},
}

// scatterOffsets are days-ago on which a seeded operator also finished a session,
// beyond their recent consecutive streak - so the profile activity heatmap reads
// as a real, sparse history rather than one solid recent block. Chosen with every
// pairwise gap >=5 and every value > the longest streak (14), so these days never
// sit adjacent to the streak run and never form a run of their own: they add
// texture without changing anyone's longest-streak number. Each operator uses the
// prefix that fits inside their tenure, so older accounts show a denser calendar.
var scatterOffsets = []int{16, 23, 31, 38, 47, 58, 66, 79, 91, 105, 118, 131}

// SeedDefaultUsers plants the showcase operators (with their completion and
// activity history) ONLY into a fresh database - one with no user accounts yet.
// The moment any account exists (the seeds themselves after the first boot, or
// real sign-ups on an established deployment) it returns without touching a
// thing. That is deliberate: it used to insert-if-absent by fixed id on every
// boot, which silently resurrected any seed operator an admin had deleted - now
// a deletion sticks. Timestamps are anchored once in Go so a streak's days stay
// consecutive even if the statements straddle midnight.
func SeedDefaultUsers(ctx context.Context, pool *pgxpool.Pool) error {
	// Only seed an empty roster. On any DB that already has accounts we leave the
	// showcase exactly as the admins have curated it - including gone, if they
	// removed it - rather than re-creating deleted rows on the next restart.
	var anyUser bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users)`).Scan(&anyUser); err != nil {
		return fmt.Errorf("seed: check for existing users: %w", err)
	}
	if anyUser {
		return nil
	}

	domain := seedEmailDomain()
	now := time.Now().UTC()

	// The most recent seeded completion lands ~10 days ago (the prior anchor);
	// earlier ones are spread back across the operator's tenure so the
	// leaderboard's labs-trend sparkline shows a real climb, not one instant.
	lastCompletedAt := now.AddDate(0, 0, -10)

	for _, u := range seedUsers {
		email := u.username + "@" + domain
		createdAt := now.AddDate(0, -u.joinedMonthsAgo, 0)
		if _, err := pool.Exec(ctx,
			`INSERT INTO users (id, email, username, oauth_provider, role, blocked, created_at)
			 VALUES ($1, $2, $3, 'seed', 'user', false, $4)
			 ON CONFLICT (id) DO NOTHING`,
			u.id, email, u.username, createdAt); err != nil {
			return fmt.Errorf("seed user %s: %w", u.username, err)
		}

		// One completion row per cleared lab, spread evenly from a week after the
		// operator joined up to lastCompletedAt so their completions carry a real
		// timeline (a single-clear operator lands their one completion at the end).
		// Deterministic slug order keeps each lab's assigned date stable across
		// re-seeds. completed_at makes each count; solve_seconds is frozen to the
		// card's best-time math (started_at trails it by that span). The upsert
		// re-syncs the timeline columns only (never answers), so an existing seed
		// board that predates the spread is corrected on the next boot.
		slugs := make([]string, 0, len(u.solves))
		for slug := range u.solves {
			slugs = append(slugs, slug)
		}
		sort.Strings(slugs)
		firstCompletedAt := createdAt.AddDate(0, 0, 7)
		for i, slug := range slugs {
			secs := u.solves[slug]
			completedAt := lastCompletedAt
			if len(slugs) > 1 {
				frac := float64(i) / float64(len(slugs)-1)
				completedAt = firstCompletedAt.Add(time.Duration(frac * float64(lastCompletedAt.Sub(firstCompletedAt))))
			}
			startedAt := completedAt.Add(-time.Duration(secs) * time.Second)
			if _, err := pool.Exec(ctx,
				`INSERT INTO lab_progress (user_id, module_slug, answers, completed_at, started_at, solve_seconds, updated_at)
				 VALUES ($1, $2, '{}'::jsonb, $3, $4, $5, $3)
				 ON CONFLICT (user_id, module_slug) DO UPDATE
				   SET completed_at  = EXCLUDED.completed_at,
				       started_at    = EXCLUDED.started_at,
				       solve_seconds = EXCLUDED.solve_seconds,
				       updated_at    = EXCLUDED.updated_at`,
				u.id, slug, completedAt, startedAt, secs); err != nil {
				return fmt.Errorf("seed progress %s/%s: %w", u.username, slug, err)
			}
		}

		// One finished session per day for streakDays consecutive days - exactly
		// what the longest-streak query reads.
		for k := 0; k < u.streakDays; k++ {
			ended := now.AddDate(0, 0, -k)
			started := ended.Add(-30 * time.Minute)
			sessionID := fmt.Sprintf("seed-%s-usage-%d", u.username, k)
			if _, err := pool.Exec(ctx,
				`INSERT INTO lab_usage (session_id, user_id, module_slug, started_at, ended_at, gib_minutes, ended_reason)
				 VALUES ($1, $2, 'ns-escape', $3, $4, 30, 'stopped')
				 ON CONFLICT (session_id) DO NOTHING`,
				sessionID, u.id, started, ended); err != nil {
				return fmt.Errorf("seed usage %s: %w", u.username, err)
			}
		}

		// Scattered earlier activity, so the profile heatmap isn't just the recent
		// streak block. Only days that fall within this operator's tenure are
		// planted (an account can't have activity before it joined), which makes an
		// older account's calendar denser than a younger one's on its own.
		for _, off := range scatterOffsets {
			ended := now.AddDate(0, 0, -off)
			if !ended.After(createdAt) {
				continue
			}
			started := ended.Add(-30 * time.Minute)
			sessionID := fmt.Sprintf("seed-%s-act-%d", u.username, off)
			if _, err := pool.Exec(ctx,
				`INSERT INTO lab_usage (session_id, user_id, module_slug, started_at, ended_at, gib_minutes, ended_reason)
				 VALUES ($1, $2, 'ns-escape', $3, $4, 30, 'stopped')
				 ON CONFLICT (session_id) DO NOTHING`,
				sessionID, u.id, started, ended); err != nil {
				return fmt.Errorf("seed activity %s: %w", u.username, err)
			}
		}
	}
	return nil
}
