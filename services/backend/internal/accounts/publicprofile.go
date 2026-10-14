package accounts

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DayActivity is one UTC calendar day's finished-session count - the same
// lab_usage signal the streak reads (a row per ended session, see db/schema.go),
// aggregated per day. Only days with at least one session appear; the frontend
// fills the gaps when it draws the contribution grid. This is real activity, not
// a generated shape: an operator who ran nothing for a month leaves that month
// empty.
type DayActivity struct {
	Date  string `json:"date"` // YYYY-MM-DD (UTC)
	Count int    `json:"count"`
}

// PublicProfile is the everyone-can-see view of an operator: identity, the
// free-text fields they chose to publish, their leaderboard standing, and the
// daily activity calendar. It draws exactly the disclosure line the public
// leaderboard does - handle and real stats, never an email or session detail -
// so linking to it from a public page leaks nothing the board didn't already.
type PublicProfile struct {
	Username            string    `json:"username"`
	Bio                 string    `json:"bio"`
	Website             string    `json:"website"`
	X                   string    `json:"x,omitempty"`
	LinkedIn            string    `json:"linkedin,omitempty"`
	HasAvatar           bool      `json:"hasAvatar"`
	JoinedAt            time.Time `json:"joinedAt"`
	Rank                int       `json:"rank"`
	TotalOperators      int       `json:"totalOperators"`
	LabsCompleted       int       `json:"labsCompleted"`
	FastestSolveSeconds *int      `json:"fastestSolveSeconds"`
	LongestStreakDays   int       `json:"longestStreakDays"`
	// Activity is the trailing ActivityDays of daily session counts (active days
	// only, oldest first). ActivityDays is the window length so the frontend can
	// size the grid without guessing.
	Activity     []DayActivity `json:"activity"`
	ActivityDays int           `json:"activityDays"`
}

// GetPublicProfile assembles one operator's public view by username. Rank and
// the headline stats come from the same ranked board the leaderboard serves, so
// a profile and the board can never disagree. Returns (nil, nil) when no such
// operator exists or they're blocked - a blocked account is absent from the
// board, so it simply isn't found here either.
func GetPublicProfile(ctx context.Context, pool *pgxpool.Pool, username string, activityDays int) (*PublicProfile, error) {
	board, err := Leaderboard(ctx, pool)
	if err != nil {
		return nil, err
	}
	for i := range board {
		if board[i].Username == username {
			return buildProfile(ctx, pool, board[i], i+1, len(board), activityDays)
		}
	}
	return nil, nil
}

// TopProfiles returns the top n operators' public profiles, ranked, for the
// landing showcase. One board read backs every profile, so the showcase agrees
// with the leaderboard by construction.
func TopProfiles(ctx context.Context, pool *pgxpool.Pool, n, activityDays int) ([]PublicProfile, error) {
	board, err := Leaderboard(ctx, pool)
	if err != nil {
		return nil, err
	}
	if n > len(board) {
		n = len(board)
	}
	out := make([]PublicProfile, 0, n)
	for i := 0; i < n; i++ {
		p, err := buildProfile(ctx, pool, board[i], i+1, len(board), activityDays)
		if err != nil {
			return nil, err
		}
		if p != nil {
			out = append(out, *p)
		}
	}
	return out, nil
}

// buildProfile fills a PublicProfile from an already-ranked board entry plus the
// user's own profile fields and activity. The entry supplies the stats (so they
// match the board); the two follow-up reads add the self-published fields and
// the calendar.
func buildProfile(ctx context.Context, pool *pgxpool.Pool, e LeaderboardEntry, rank, total, activityDays int) (*PublicProfile, error) {
	p := PublicProfile{
		Username:            e.Username,
		Rank:                rank,
		TotalOperators:      total,
		LabsCompleted:       e.LabsCompleted,
		FastestSolveSeconds: e.FastestSolveSeconds,
		LongestStreakDays:   e.LongestStreakDays,
		ActivityDays:        activityDays,
	}
	err := pool.QueryRow(ctx,
		`SELECT bio, website, x_url, linkedin_url, avatar IS NOT NULL, created_at FROM users WHERE id = $1`,
		e.UserID,
	).Scan(&p.Bio, &p.Website, &p.X, &p.LinkedIn, &p.HasAvatar, &p.JoinedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	act, err := dailyActivity(ctx, pool, e.UserID, activityDays)
	if err != nil {
		return nil, err
	}
	p.Activity = act
	return &p, nil
}

// dailyActivity returns per-UTC-day finished-session counts for one user over
// the trailing `days` days, oldest first, days with no session omitted. Same
// source and day-bucketing as the streak (see longestStreaks) so the calendar
// and the streak number are telling the same story.
func dailyActivity(ctx context.Context, pool *pgxpool.Pool, userID string, days int) ([]DayActivity, error) {
	since := time.Now().UTC().AddDate(0, 0, -days)
	rows, err := pool.Query(ctx, `
		SELECT (date_trunc('day', ended_at AT TIME ZONE 'UTC'))::date AS day, COUNT(*)
		FROM lab_usage
		WHERE user_id = $1 AND ended_at >= $2
		GROUP BY day
		ORDER BY day`, userID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []DayActivity{}
	for rows.Next() {
		var day time.Time
		var count int
		if err := rows.Scan(&day, &count); err != nil {
			return nil, err
		}
		out = append(out, DayActivity{Date: day.Format("2006-01-02"), Count: count})
	}
	return out, rows.Err()
}

// GetAvatarByUsername serves the public-profile avatar. It resolves the handle
// to a non-blocked account and returns pgx.ErrNoRows for an unknown or blocked
// one, so a blocked operator's image can't be pulled by guessing their name.
func GetAvatarByUsername(ctx context.Context, pool *pgxpool.Pool, username string) ([]byte, string, error) {
	var id string
	err := pool.QueryRow(ctx,
		`SELECT id FROM users WHERE username = $1 AND blocked = false`, username,
	).Scan(&id)
	if err != nil {
		return nil, "", err
	}
	return GetAvatar(ctx, pool, id)
}
