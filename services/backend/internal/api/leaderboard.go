package api

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"backend/internal/accounts"
)

// publicReveal is how many ranks a signed-out (or signed-in-but-not-yet-cleared)
// visitor sees. Everything past it is gated behind clearing a lab - which is
// also what keeps the page from ever disclosing how many operators are ranked:
// a locked board shows the leaders and a redacted tail, never a total.
const publicReveal = 3

type leaderboardEntryDTO struct {
	Rank int `json:"rank"`
	// UserID is an internal account id and is only sent to a signed-in caller
	// (who can actually use it - avatars are auth-gated). A signed-out visitor's
	// board is keyed by rank alone, so no internal identifier rides a public
	// payload. omitempty drops it entirely when unset.
	UserID    string `json:"userId,omitempty"`
	Username  string `json:"username"`
	HasAvatar bool   `json:"hasAvatar"`
	// Public profile links the operator added in Settings, shown as icons on the
	// board. Server-validated to https on the expected hosts; omitempty when unset.
	X                   string `json:"x,omitempty"`
	LinkedIn            string `json:"linkedin,omitempty"`
	LabsCompleted       int    `json:"labsCompleted"`
	FastestSolveSeconds *int   `json:"fastestSolveSeconds"`
	LongestStreakDays   int    `json:"longestStreakDays"`
	// LabsTrend is the cumulative labs-cleared sparkline series (see
	// accounts.LeaderboardEntry). null for an operator with no completions.
	LabsTrend []int `json:"labsTrend"`
	// Distinction is a category slug ("fastest escape" / "longest streak" /
	// "most labs cleared") for the holder of that superlative; empty otherwise.
	// The frontend pairs it with the entry's own number for the flavor line.
	Distinction string `json:"distinction,omitempty"`
	IsViewer    bool   `json:"isViewer,omitempty"`
}

type leaderboardViewer struct {
	Authenticated bool `json:"authenticated"`
	HasClearedLab bool `json:"hasClearedLab"`
	Rank          *int `json:"rank,omitempty"`
	// LabsCompleted is the viewer's own count, sent so the sidebar climb widget
	// can frame "N labs behind" without the viewer's row being in the revealed
	// slice (a locked board only reveals the top ranks, never the viewer's tail
	// position). Zero for a viewer not found on the board.
	LabsCompleted int `json:"labsCompleted"`
}

type leaderboardResponse struct {
	Entries  []leaderboardEntryDTO `json:"entries"`
	Revealed int                   `json:"revealed"`
	Locked   bool                  `json:"locked"`
	Viewer   leaderboardViewer     `json:"viewer"`
	// NextTarget is the operator the climb widget aims the viewer at - a genuine
	// step up rather than whoever they are level with (see climbTargetIdx: one
	// lab-tier beyond the immediate neighbour, with a medalist carve-out).
	// Computed over the FULL board even when it's locked, so a not-yet-on-the-board
	// viewer still gets a real target rather than the far-off #1. nil when no one
	// is ahead (the viewer leads) or there's no authenticated viewer. It names one
	// operator; it never discloses the board's size or tail.
	NextTarget *leaderboardEntryDTO `json:"nextTarget,omitempty"`
}

// leaderboard godoc
// @Summary     The operator leaderboard
// @Description Public. Signed-out visitors and signed-in users who haven't cleared a lab see the top ranks only (the tail is gated, and the total is never disclosed). A signed-in user who has completed a lab sees the full board and their own rank.
// @Tags        labs
// @Produce     json
// @Success     200 {object} leaderboardResponse
// @Router      /leaderboard [get]
func leaderboard(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		board, err := accounts.Leaderboard(r.Context(), pool)
		if err != nil {
			http.Error(w, "failed to load leaderboard", http.StatusInternalServerError)
			return
		}

		viewer := optionalUser(pool, r)
		resp := leaderboardResponse{Viewer: leaderboardViewer{Authenticated: viewer != nil}}

		// Locate the viewer on the board. Everyone is on it now (including zero-stat
		// users), so presence alone isn't the unlock - clearing a lab is. Their rank
		// is only surfaced once unlocked, so a locked visitor's position can't be
		// used to infer how many operators exist.
		viewerIdx := -1
		if viewer != nil {
			for i := range board {
				if board[i].UserID == viewer.ID {
					viewerIdx = i
					break
				}
			}
		}
		hasCleared := viewerIdx >= 0 && board[viewerIdx].LabsCompleted > 0
		if viewer != nil {
			resp.Viewer.HasClearedLab = hasCleared
			if hasCleared {
				rk := viewerIdx + 1
				resp.Viewer.Rank = &rk
			}
			if viewerIdx >= 0 {
				resp.Viewer.LabsCompleted = board[viewerIdx].LabsCompleted
				if ti := climbTargetIdx(board, viewerIdx); ti >= 0 {
					t := board[ti]
					resp.NextTarget = &leaderboardEntryDTO{
						Rank:                ti + 1,
						UserID:              t.UserID,
						Username:            t.Username,
						HasAvatar:           t.HasAvatar,
						X:                   t.X,
						LinkedIn:            t.LinkedIn,
						LabsCompleted:       t.LabsCompleted,
						FastestSolveSeconds: t.FastestSolveSeconds,
						LongestStreakDays:   t.LongestStreakDays,
						LabsTrend:           t.LabsTrend,
					}
				}
			}
		}

		// Superlative holders, computed over the WHOLE board so a revealed row's
		// flavor line is true even when the tail is hidden.
		fastestIdx, streakIdx := -1, -1
		for i := range board {
			if board[i].FastestSolveSeconds != nil &&
				(fastestIdx < 0 || *board[i].FastestSolveSeconds < *board[fastestIdx].FastestSolveSeconds) {
				fastestIdx = i
			}
			if board[i].LongestStreakDays > 0 &&
				(streakIdx < 0 || board[i].LongestStreakDays > board[streakIdx].LongestStreakDays) {
				streakIdx = i
			}
		}

		unlocked := hasCleared
		resp.Locked = !unlocked

		limit := len(board)
		if !unlocked && limit > publicReveal {
			limit = publicReveal
		}
		resp.Revealed = limit

		resp.Entries = make([]leaderboardEntryDTO, 0, limit)
		for i := 0; i < limit; i++ {
			e := board[i]
			dto := leaderboardEntryDTO{
				Rank:                i + 1,
				Username:            e.Username,
				HasAvatar:           e.HasAvatar,
				X:                   e.X,
				LinkedIn:            e.LinkedIn,
				LabsCompleted:       e.LabsCompleted,
				FastestSolveSeconds: e.FastestSolveSeconds,
				LongestStreakDays:   e.LongestStreakDays,
				LabsTrend:           e.LabsTrend,
				IsViewer:            viewer != nil && e.UserID == viewer.ID,
			}
			// Only a signed-in caller receives the internal id (see the DTO).
			if viewer != nil {
				dto.UserID = e.UserID
			}
			switch i {
			case fastestIdx:
				dto.Distinction = "fastest escape"
			case streakIdx:
				dto.Distinction = "longest streak"
			default:
				// Only the actual leader, and only if they've cleared something -
				// "most labs cleared" over a board where the top row is still on 0
				// would be a nonsense badge.
				if i == 0 && e.LabsCompleted > 0 {
					dto.Distinction = "most labs cleared"
				}
			}
			resp.Entries = append(resp.Entries, dto)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// climbTargetIdx picks the operator the "Next up" climb widget aims the viewer
// at, returning its board index (or -1 when the viewer leads or isn't on the
// board). board is ranked best-first, so a lower index is a better rank.
//
// The aim is a genuine step up, not whoever the viewer is basically level with.
// So rather than the nearest operator with more labs, it targets one lab-tier
// BEYOND the operator immediately above the viewer: the nearest operator with
// strictly more labs than that neighbour. When the neighbour is merely tied with
// the viewer this reduces to "the nearest operator with more labs than me" - the
// old behaviour - because the neighbour's count is the viewer's own.
//
// Two carve-outs:
//   - If the operator immediately above is a MEDALIST (top 3, and actually on the
//     board with a cleared lab), aim straight at them - you don't leapfrog a
//     medalist; overtaking them is the climb.
//   - Fallback: if no one sits a full tier above the neighbour (they top their
//     tier), aim at the nearest operator with more labs than the viewer, so a tie
//     is never surfaced as the target and there's always a target while anyone is
//     ahead.
func climbTargetIdx(board []accounts.LeaderboardEntry, viewerIdx int) int {
	if viewerIdx <= 0 {
		return -1
	}
	immediate := viewerIdx - 1

	// Medalist neighbour (index 0..2 == rank 1..3, with a real solve): target them.
	if immediate <= 2 && board[immediate].LabsCompleted > 0 {
		return immediate
	}

	// One tier beyond the neighbour: nearest operator with more labs than them.
	threshold := board[immediate].LabsCompleted
	for i := immediate; i >= 0; i-- {
		if board[i].LabsCompleted > threshold {
			return i
		}
	}

	// Fallback: nobody a full tier above the neighbour. Nearest operator with
	// strictly more labs than the viewer (never a tie).
	me := board[viewerIdx].LabsCompleted
	for i := immediate; i >= 0; i-- {
		if board[i].LabsCompleted > me {
			return i
		}
	}
	return -1
}
