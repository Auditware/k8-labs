package api

import (
	"testing"

	"backend/internal/accounts"
)

// climbTargetIdx aims the "Next up" widget one lab-tier beyond the operator
// immediately above the viewer, with a medalist carve-out and a no-tie fallback.
// board is ranked best-first (index 0 = rank 1).
func TestClimbTargetIdx(t *testing.T) {
	// labs builds a best-first board from a list of lab counts.
	labs := func(counts ...int) []accounts.LeaderboardEntry {
		b := make([]accounts.LeaderboardEntry, len(counts))
		for i, c := range counts {
			b[i] = accounts.LeaderboardEntry{LabsCompleted: c}
		}
		return b
	}

	cases := []struct {
		name      string
		board     []accounts.LeaderboardEntry
		viewerIdx int
		want      int
	}{
		{
			// Viewer leads - nobody to chase.
			name: "viewer is rank 1", board: labs(5, 3, 1), viewerIdx: 0, want: -1,
		},
		{
			// Neighbour (#5, idx3, non-medalist) is tied with the viewer, so "one tier
			// beyond the neighbour" == "more labs than me": skip the tie, take the next
			// up. idx: 0=7,1=6,2=5,3=5(neighbour),4=5(me). nearest >5 = idx1 (6).
			name:  "neighbour tied - skips to next with more labs",
			board: labs(7, 6, 5, 5, 5), viewerIdx: 4, want: 1,
		},
		{
			// Neighbour (#4, idx3, non-medalist) has MORE labs than the viewer (not
			// tied). Aim one tier beyond the neighbour, not at the neighbour.
			// idx: 0=9,1=8,2=7,3=6(neighbour),4=4(me). beyond 6 -> nearest >6 = idx2.
			name:  "neighbour ahead - aims one tier beyond",
			board: labs(9, 8, 7, 6, 4), viewerIdx: 4, want: 2,
		},
		{
			// The operator immediately above is a medalist (top 3) with a solve:
			// target them directly, don't leapfrog.
			// viewer is rank 4 (idx3); neighbour idx2 == rank 3 (bronze).
			name:  "medalist neighbour - target them",
			board: labs(9, 8, 7, 3), viewerIdx: 3, want: 2,
		},
		{
			// Everyone above is tied with the viewer (non-medalist neighbour at idx4),
			// so nobody has more labs - no target, the widget falls to "Climb the board".
			name:  "everyone above tied with viewer - no target",
			board: labs(5, 5, 5, 5, 5), viewerIdx: 4, want: -1,
		},
		{
			// Fallback path: neighbour (idx3, 6 labs) is ahead of viewer (idx4, 4
			// labs) but tops the board's tier (nobody above has >6). Aim at the
			// neighbour. board: 6,6,6,6,4 -> viewer idx4; threshold 6, none >6;
			// fallback nearest >4 = idx3.
			name:  "neighbour tops tier - fallback to neighbour",
			board: labs(6, 6, 6, 6, 4), viewerIdx: 4, want: 3,
		},
		{
			// A medalist neighbour on zero labs is not really a medalist (matches the
			// board's own medal rule), so the carve-out doesn't fire; with everyone on
			// zero there's no one to chase.
			name:  "zero-lab top-3 neighbour is not a medalist",
			board: labs(0, 0, 0, 0), viewerIdx: 3, want: -1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := climbTargetIdx(tc.board, tc.viewerIdx); got != tc.want {
				t.Errorf("climbTargetIdx = %d, want %d", got, tc.want)
			}
		})
	}
}
