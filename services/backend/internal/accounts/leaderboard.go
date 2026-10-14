package accounts

import (
	"context"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// LeaderboardEntry is one ranked operator with their real stats. Every
// non-blocked user appears, including those who haven't completed anything yet -
// they simply carry zero stats and sort to the bottom, so the board shows the
// roster from day one rather than an empty state.
//
// FastestSolveSeconds is the shortest solve across their completed labs, where a
// lab's solve time is the span lab_progress.started_at -> completed_at (see
// db/schema.go). It is nil for a user with no completed labs (or whose completed
// labs all predate solve tracking). LongestStreakDays is the longest run of
// consecutive UTC days on which they ended a lab session.
type LeaderboardEntry struct {
	UserID              string `json:"userId"`
	Username            string `json:"username"`
	HasAvatar           bool   `json:"hasAvatar"`
	X                   string `json:"x,omitempty"`        // https x.com/twitter.com URL, or ""
	LinkedIn            string `json:"linkedin,omitempty"` // https linkedin.com URL, or ""
	LabsCompleted       int    `json:"labsCompleted"`
	FastestSolveSeconds *int   `json:"fastestSolveSeconds"`
	LongestStreakDays   int    `json:"longestStreakDays"`
	// LabsTrend is this operator's cumulative labs-cleared sampled at a fixed set
	// of evenly spaced points across a window shared by the whole board (the
	// earliest completion on the board -> now). It's the honest climb behind
	// LabsCompleted - derived from the same completion records, never invented -
	// and the shared window lets the frontend's sparklines line up on one x-axis
	// so a busier operator's curve genuinely differs. nil for an operator with no
	// completions.
	LabsTrend []int `json:"labsTrend"`
}

// trendSamples is how many points each LabsTrend series carries. Enough to read
// as a curve, few enough to stay a glanceable sparkline.
const trendSamples = 10

// Leaderboard returns every non-blocked user, ranked: most labs first, then
// fastest solve, then longest streak, then username as a stable tiebreak. Users
// with no completions sort to the bottom (0 labs, no solve) rather than being
// dropped, so the board is the live roster. Blocked users are excluded so a
// blocked account never sits on a public board. The LEFT JOIN is what keeps a
// user who has never opened a lab (no lab_progress row) on the board at zero.
func Leaderboard(ctx context.Context, pool *pgxpool.Pool) ([]LeaderboardEntry, error) {
	// The >= 15s floor on `fastest` drops implausibly-fast solve_seconds: if a
	// user submitted the flag before any progress row existed, the row was created
	// at submission time so started_at ~ completed_at and solve_seconds floored to
	// 1 (a bogus "0:01"). Real solves clear the floor easily; the root cause is
	// also fixed by stamping started_at at lab-session start (see createSession).
	rows, err := pool.Query(ctx, `
		SELECT u.id, u.username, (u.avatar IS NOT NULL) AS has_avatar, u.x_url, u.linkedin_url,
		       COUNT(*) FILTER (WHERE p.completed_at IS NOT NULL) AS labs_completed,
		       MIN(p.solve_seconds) FILTER (WHERE p.completed_at IS NOT NULL AND p.solve_seconds >= 15) AS fastest
		FROM users u
		LEFT JOIN lab_progress p ON p.user_id = u.id
		WHERE u.blocked = false
		GROUP BY u.id, u.username, has_avatar`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := []LeaderboardEntry{}
	for rows.Next() {
		var e LeaderboardEntry
		var fastest *int32
		if err := rows.Scan(&e.UserID, &e.Username, &e.HasAvatar, &e.X, &e.LinkedIn, &e.LabsCompleted, &fastest); err != nil {
			return nil, err
		}
		if fastest != nil {
			v := int(*fastest)
			e.FastestSolveSeconds = &v
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	streaks, err := longestStreaks(ctx, pool)
	if err != nil {
		return nil, err
	}
	for i := range entries {
		entries[i].LongestStreakDays = streaks[entries[i].UserID]
	}

	trends, err := completionTrends(ctx, pool)
	if err != nil {
		return nil, err
	}
	for i := range entries {
		entries[i].LabsTrend = trends[entries[i].UserID]
	}

	sort.SliceStable(entries, func(i, j int) bool { return rankLess(entries[i], entries[j]) })
	return entries, nil
}

// completionTrends builds each user's LabsTrend: their cumulative labs-cleared
// sampled across a window shared by everyone with a completion (earliest
// completion on the board -> now). A shared window is what makes the sparklines
// comparable row to row - the same span of real time under every curve - and
// keeps a recent-but-idle operator's line honestly flattening out toward now
// rather than being rescaled to hide the gap. Users with no completions get no
// series (they never appear in the query, so the map has no entry for them).
func completionTrends(ctx context.Context, pool *pgxpool.Pool) (map[string][]int, error) {
	rows, err := pool.Query(ctx, `
		SELECT user_id, completed_at
		FROM lab_progress
		WHERE completed_at IS NOT NULL
		ORDER BY user_id, completed_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byUser := map[string][]time.Time{}
	var start time.Time
	for rows.Next() {
		var uid string
		var at time.Time
		if err := rows.Scan(&uid, &at); err != nil {
			return nil, err
		}
		byUser[uid] = append(byUser[uid], at)
		if start.IsZero() || at.Before(start) {
			start = at
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	end := time.Now().UTC()
	out := make(map[string][]int, len(byUser))
	for uid, times := range byUser {
		out[uid] = bucketCumulative(times, start, end, trendSamples)
	}
	return out, nil
}

// bucketCumulative samples a cumulative event count at `samples` evenly spaced
// boundaries across (start, end]. Boundary k (1-based) sits at
// start + k/samples*(end-start), and its value is how many of `times` fall at or
// before it; the series is therefore monotonic non-decreasing and its last value
// is the total. `times` must be ascending. Returns nil for samples<=0 or no
// times; a degenerate window (end not after start) yields a flat series at the
// total so a single-instant history still reads as "already there".
func bucketCumulative(times []time.Time, start, end time.Time, samples int) []int {
	if samples <= 0 || len(times) == 0 {
		return nil
	}
	total := len(times)
	out := make([]int, samples)
	span := end.Sub(start)
	if span <= 0 {
		for i := range out {
			out[i] = total
		}
		return out
	}
	for k := 1; k <= samples; k++ {
		boundary := start.Add(time.Duration(int64(span) * int64(k) / int64(samples)))
		c := 0
		for _, t := range times {
			if t.After(boundary) {
				break
			}
			c++
		}
		out[k-1] = c
	}
	return out
}

// rankLess orders the board: most labs first, then longest streak, then fastest
// solve, then username as a stable tiebreak so equal operators don't shuffle
// between reads. Streak is deliberately ranked ABOVE speed - consistent daily
// practice is valued over a single fast run, and it also keeps a fluke/short
// solve time from vaulting someone over a steadier operator.
func rankLess(a, b LeaderboardEntry) bool {
	if a.LabsCompleted != b.LabsCompleted {
		return a.LabsCompleted > b.LabsCompleted
	}
	if a.LongestStreakDays != b.LongestStreakDays {
		return a.LongestStreakDays > b.LongestStreakDays
	}
	// Then fastest solve: a recorded time beats none; between two, faster wins.
	if (a.FastestSolveSeconds == nil) != (b.FastestSolveSeconds == nil) {
		return a.FastestSolveSeconds != nil
	}
	if a.FastestSolveSeconds != nil && b.FastestSolveSeconds != nil && *a.FastestSolveSeconds != *b.FastestSolveSeconds {
		return *a.FastestSolveSeconds < *b.FastestSolveSeconds
	}
	return a.Username < b.Username
}

// longestStreaks computes, per user, the longest run of consecutive UTC days on
// which a lab session ended. Derived, not stored: lab_usage only gains a row
// when a session ends, so a day with only an unfinished session doesn't count
// until it does. Days are distinct and ordered per user, so consecutive
// calendar days differ by exactly 24h (UTC has no DST to perturb that).
func longestStreaks(ctx context.Context, pool *pgxpool.Pool) (map[string]int, error) {
	rows, err := pool.Query(ctx, `
		SELECT user_id, date_trunc('day', ended_at AT TIME ZONE 'UTC')::date AS day
		FROM lab_usage
		GROUP BY user_id, day
		ORDER BY user_id, day`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byUser := map[string][]time.Time{}
	for rows.Next() {
		var uid string
		var day time.Time
		if err := rows.Scan(&uid, &day); err != nil {
			return nil, err
		}
		byUser[uid] = append(byUser[uid], day)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make(map[string]int, len(byUser))
	for uid, days := range byUser {
		out[uid] = longestConsecutiveDays(days)
	}
	return out, nil
}

// longestConsecutiveDays returns the longest run of consecutive calendar days in
// a set of distinct, ascending UTC day-timestamps (midnight-aligned, as the
// query produces). Consecutive days differ by exactly 24h; any other gap breaks
// the run.
func longestConsecutiveDays(days []time.Time) int {
	best, run := 0, 0
	var prev time.Time
	for i, d := range days {
		if i == 0 || d.Sub(prev) != 24*time.Hour {
			run = 1
		} else {
			run++
		}
		prev = d
		if run > best {
			best = run
		}
	}
	return best
}
