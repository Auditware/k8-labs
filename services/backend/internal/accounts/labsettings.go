package accounts

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"backend/internal/labs"
)

// LabSetting is one module's operator-tunable knobs (Admin -> Lab Management).
// A module with no row runs on labs.DefaultTiming; IsDefault says which case a
// given response is, so the admin UI can show "default (60m)" rather than
// pretending someone chose it.
type LabSetting struct {
	ModuleSlug      string     `json:"moduleSlug"`
	DurationMinutes int        `json:"durationMinutes"`
	IdleMinutes     int        `json:"idleMinutes"`
	IsDefault       bool       `json:"isDefault"`
	UpdatedAt       *time.Time `json:"updatedAt,omitempty"`
	UpdatedBy       *string    `json:"updatedBy,omitempty"`
}

// TimingStore reads lab_settings for the session manager. It satisfies
// labs.TimingSource, which is why internal/labs needs no database dependency of
// its own.
//
// Reads are cached in-process: TimingFor is called on the session-create path
// and would otherwise put a database round trip in front of every lab start for
// a value that changes maybe twice a year. The cache is invalidated on write
// (SetLabSetting/ResetLabSetting), so an admin's change takes effect on the next
// session immediately - it just isn't visible to a *different* backend replica
// until its own TTL lapses. Acceptable: the value only decides how long a new
// session lives, and there is one replica today (see Manager's doc).
type TimingStore struct {
	pool  *pgxpool.Pool
	cache *timingCache
}

func NewTimingStore(pool *pgxpool.Pool) *TimingStore {
	return &TimingStore{pool: pool, cache: newTimingCache(timingCacheTTL)}
}

const timingCacheTTL = 60 * time.Second

// TimingFor resolves a module's limits, falling back to the module's own
// declared default for a slug with no row (labs.ModuleTiming, which itself
// falls through to labs.DefaultTiming). Never returns an error: a database blip
// must not block a student from starting a lab, so it degrades to that default
// and logs nothing louder. Callers clamp the result (see labs.Timing.Clamp).
func (s *TimingStore) TimingFor(moduleSlug string) labs.Timing {
	if s == nil || s.pool == nil {
		return labs.ModuleTiming(moduleSlug)
	}
	if t, ok := s.cache.get(moduleSlug); ok {
		return t
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var durMin, idleMin int
	err := s.pool.QueryRow(ctx,
		`SELECT duration_minutes, idle_minutes FROM lab_settings WHERE module_slug = $1`,
		moduleSlug).Scan(&durMin, &idleMin)
	t := labs.ModuleTiming(moduleSlug)
	if err == nil {
		t = labs.Timing{
			MaxAge: time.Duration(durMin) * time.Minute,
			Idle:   time.Duration(idleMin) * time.Minute,
		}.Clamp()
	} else if !errors.Is(err, pgx.ErrNoRows) {
		// A real failure (not "no override"). Cache nothing so the next start
		// retries rather than pinning the default for a whole TTL.
		return labs.ModuleTiming(moduleSlug)
	}
	s.cache.put(moduleSlug, t)
	return t
}

// ListLabSettings returns the effective setting for every slug passed in, row
// or not - so the admin UI renders one line per module without having to know
// which ones have been customised.
func ListLabSettings(ctx context.Context, pool *pgxpool.Pool, slugs []string) ([]LabSetting, error) {
	rows, err := pool.Query(ctx,
		`SELECT module_slug, duration_minutes, idle_minutes, updated_at, updated_by FROM lab_settings`)
	if err != nil {
		return nil, fmt.Errorf("list lab settings: %w", err)
	}
	defer rows.Close()

	overrides := map[string]LabSetting{}
	for rows.Next() {
		var ls LabSetting
		if err := rows.Scan(&ls.ModuleSlug, &ls.DurationMinutes, &ls.IdleMinutes, &ls.UpdatedAt, &ls.UpdatedBy); err != nil {
			return nil, err
		}
		overrides[ls.ModuleSlug] = ls
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]LabSetting, 0, len(slugs))
	for _, slug := range slugs {
		if ls, ok := overrides[slug]; ok {
			out = append(out, ls)
			continue
		}
		// "Default" here means the module's own declared timing, not the global
		// fallback - that is what actually runs for a slug with no row, so it is
		// what the admin UI has to show.
		def := labs.ModuleTiming(slug)
		out = append(out, LabSetting{
			ModuleSlug:      slug,
			DurationMinutes: int(def.MaxAge / time.Minute),
			IdleMinutes:     int(def.Idle / time.Minute),
			IsDefault:       true,
		})
	}
	return out, nil
}

// SetLabSetting upserts a module's override. Validation lives here rather than
// only at the handler so any future caller gets it too: the bounds are the same
// ones labs.Timing.Clamp enforces, but rejecting loudly beats silently clamping
// an admin's typo to something they didn't ask for.
func SetLabSetting(ctx context.Context, pool *pgxpool.Pool, store *TimingStore, moduleSlug string, durationMinutes, idleMinutes int, updatedBy string) error {
	minM := int(labs.MinDuration / time.Minute)
	maxM := int(labs.MaxDuration / time.Minute)
	if durationMinutes < minM || durationMinutes > maxM {
		return fmt.Errorf("duration must be between %d and %d minutes", minM, maxM)
	}
	if idleMinutes < minM || idleMinutes > maxM {
		return fmt.Errorf("idle timeout must be between %d and %d minutes", minM, maxM)
	}
	// An idle timeout past the hard cap can never fire, so accepting it would
	// show the admin a setting that does nothing.
	if idleMinutes > durationMinutes {
		return fmt.Errorf("idle timeout (%dm) cannot exceed the session duration (%dm)", idleMinutes, durationMinutes)
	}

	_, err := pool.Exec(ctx, `
		INSERT INTO lab_settings (module_slug, duration_minutes, idle_minutes, updated_at, updated_by)
		VALUES ($1, $2, $3, now(), $4)
		ON CONFLICT (module_slug) DO UPDATE
		SET duration_minutes = EXCLUDED.duration_minutes,
		    idle_minutes     = EXCLUDED.idle_minutes,
		    updated_at       = now(),
		    updated_by       = EXCLUDED.updated_by`,
		moduleSlug, durationMinutes, idleMinutes, updatedBy)
	if err != nil {
		return fmt.Errorf("set lab setting: %w", err)
	}
	store.invalidate(moduleSlug)
	return nil
}

// ResetLabSetting drops a module's override so it falls back to the built-in
// default. Deleting a row that isn't there is not an error.
func ResetLabSetting(ctx context.Context, pool *pgxpool.Pool, store *TimingStore, moduleSlug string) error {
	if _, err := pool.Exec(ctx, `DELETE FROM lab_settings WHERE module_slug = $1`, moduleSlug); err != nil {
		return fmt.Errorf("reset lab setting: %w", err)
	}
	store.invalidate(moduleSlug)
	return nil
}

func (s *TimingStore) invalidate(moduleSlug string) {
	if s != nil && s.cache != nil {
		s.cache.drop(moduleSlug)
	}
}
