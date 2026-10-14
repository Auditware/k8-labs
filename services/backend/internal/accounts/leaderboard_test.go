package accounts

import (
	"testing"
	"time"
)

func day(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

// The streak is the whole point of the "longest streak" stat, and the two ways
// it goes wrong are miscounting a gap as consecutive and resetting on a run that
// should continue - so both the happy runs and every kind of break are covered.
func TestLongestConsecutiveDays(t *testing.T) {
	cases := []struct {
		name string
		days []time.Time
		want int
	}{
		{"empty", nil, 0},
		{"single day", []time.Time{day("2026-07-01")}, 1},
		{"two consecutive", []time.Time{day("2026-07-01"), day("2026-07-02")}, 2},
		{"gap resets", []time.Time{day("2026-07-01"), day("2026-07-03")}, 1},
		{
			"longest run is in the middle",
			[]time.Time{day("2026-07-01"), day("2026-07-03"), day("2026-07-04"), day("2026-07-05"), day("2026-07-08")},
			3,
		},
		{
			"run then break then shorter run",
			[]time.Time{day("2026-07-01"), day("2026-07-02"), day("2026-07-03"), day("2026-07-10"), day("2026-07-11")},
			3,
		},
		{
			"crosses a month boundary",
			[]time.Time{day("2026-07-30"), day("2026-07-31"), day("2026-08-01")},
			3,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := longestConsecutiveDays(tc.days); got != tc.want {
				t.Errorf("longestConsecutiveDays(%v) = %d, want %d", tc.days, got, tc.want)
			}
		})
	}
}

// bucketCumulative backs the labs-trend sparkline, so the two ways it goes wrong
// are placing an event in the wrong bucket (the boundary math) and mishandling a
// degenerate window - both covered, alongside the shape of a normal spread.
func TestBucketCumulative(t *testing.T) {
	ts := func(ss ...string) []time.Time {
		out := make([]time.Time, len(ss))
		for i, s := range ss {
			out[i] = day(s)
		}
		return out
	}
	cases := []struct {
		name       string
		times      []time.Time
		start, end time.Time
		samples    int
		want       []int
	}{
		{"no times", nil, day("2026-01-01"), day("2026-01-11"), 5, nil},
		{"non-positive samples", ts("2026-01-02"), day("2026-01-01"), day("2026-01-11"), 0, nil},
		{
			// A degenerate (zero-width) window can't be bucketed by time, so every
			// sample reads the full total - a single-instant history is "already there".
			"degenerate window is flat at the total",
			ts("2026-01-01", "2026-01-01"),
			day("2026-01-01"), day("2026-01-01"), 4,
			[]int{2, 2, 2, 2},
		},
		{
			// Events on days 2/6/10 across a 10-day window in 5 buckets (boundaries
			// at days 3/5/7/9/11): cumulative counts step 1,1,2,2,3.
			"even spread steps up across buckets",
			ts("2026-01-03", "2026-01-07", "2026-01-11"),
			day("2026-01-01"), day("2026-01-11"), 5,
			[]int{1, 1, 2, 2, 3},
		},
		{
			// One late event lands only in the final bucket - the hockey stick a
			// single recent completion draws.
			"single late event only fills the last bucket",
			ts("2026-01-10"),
			day("2026-01-01"), day("2026-01-11"), 5,
			[]int{0, 0, 0, 0, 1},
		},
		{
			// The last sample is always the total, even when everything already
			// landed before the first boundary.
			"all events before the first boundary",
			ts("2026-01-01", "2026-01-01", "2026-01-01"),
			day("2026-01-01"), day("2026-01-11"), 5,
			[]int{3, 3, 3, 3, 3},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := bucketCumulative(tc.times, tc.start, tc.end, tc.samples)
			if len(got) != len(tc.want) {
				t.Fatalf("bucketCumulative = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("bucketCumulative = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func secs(n int) *int { return &n }

// rankLess is the tiebreak ladder the board is sorted by; each case pins one
// rung, including the two that are easy to get backwards: a recorded solve must
// outrank no-solve, and among recorded solves smaller (faster) wins.
func TestRankLess(t *testing.T) {
	cases := []struct {
		name string
		a, b LeaderboardEntry
		want bool // want a to rank before b
	}{
		{
			"more labs wins",
			LeaderboardEntry{LabsCompleted: 5},
			LeaderboardEntry{LabsCompleted: 4},
			true,
		},
		{
			"fewer labs loses",
			LeaderboardEntry{LabsCompleted: 3},
			LeaderboardEntry{LabsCompleted: 4},
			false,
		},
		{
			"equal labs, a recorded solve beats none",
			LeaderboardEntry{LabsCompleted: 4, FastestSolveSeconds: secs(600)},
			LeaderboardEntry{LabsCompleted: 4, FastestSolveSeconds: nil},
			true,
		},
		{
			"equal labs, none loses to a recorded solve",
			LeaderboardEntry{LabsCompleted: 4, FastestSolveSeconds: nil},
			LeaderboardEntry{LabsCompleted: 4, FastestSolveSeconds: secs(600)},
			false,
		},
		{
			"equal labs, faster solve wins",
			LeaderboardEntry{LabsCompleted: 4, FastestSolveSeconds: secs(120)},
			LeaderboardEntry{LabsCompleted: 4, FastestSolveSeconds: secs(300)},
			true,
		},
		{
			"labs and solve equal, longer streak wins",
			LeaderboardEntry{LabsCompleted: 4, FastestSolveSeconds: secs(120), LongestStreakDays: 9},
			LeaderboardEntry{LabsCompleted: 4, FastestSolveSeconds: secs(120), LongestStreakDays: 6},
			true,
		},
		{
			// Streak outranks speed: a longer streak wins even against a faster solve.
			"equal labs, longer streak beats a faster solve",
			LeaderboardEntry{LabsCompleted: 4, FastestSolveSeconds: secs(600), LongestStreakDays: 9},
			LeaderboardEntry{LabsCompleted: 4, FastestSolveSeconds: secs(120), LongestStreakDays: 3},
			true,
		},
		{
			"all equal, username breaks the tie",
			LeaderboardEntry{LabsCompleted: 4, FastestSolveSeconds: secs(120), LongestStreakDays: 9, Username: "amy"},
			LeaderboardEntry{LabsCompleted: 4, FastestSolveSeconds: secs(120), LongestStreakDays: 9, Username: "zoe"},
			true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rankLess(tc.a, tc.b); got != tc.want {
				t.Errorf("rankLess = %v, want %v", got, tc.want)
			}
		})
	}
}
