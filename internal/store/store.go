// Package store loads and saves the small JSON state file that gives the plant
// memory between runs (highest stage reached, best streak, last-seen commit).
//
// The on-disk format is a versioned JSON document so the schema can evolve
// without breaking older files; unknown or future versions still load on a
// best-effort basis and fall back to sane defaults. Writes are atomic (a temp
// file in the same directory is fully written, fsync'd, then renamed over the
// target) so a crash or full disk can never leave a half-written, corrupt
// state file behind.
//
// The default location is XDG-aware:
//
//	$XDG_CONFIG_HOME/commit-sprout/state.json   (when XDG_CONFIG_HOME is set)
//	~/.config/commit-sprout/state.json          (otherwise)
//
// The store deliberately depends on internal/plant only for the small State
// value it persists; plant itself performs no I/O, so the two stay cleanly
// separated (plant is the pure brain, store is the disk).
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/rwrife/commit-sprout/internal/plant"
)

// SchemaVersion is the current on-disk schema version. It is written into every
// saved file and checked on load so future changes can migrate older documents.
// Bump this whenever the persisted shape changes in a non-additive way.
const SchemaVersion = 1

// appDir is the per-user config subdirectory this tool owns.
const appDir = "commit-sprout"

// stateFile is the JSON state file name within appDir.
const stateFile = "state.json"

// dirPerm/filePerm are the permissions used for the config directory and state
// file. State is non-secret but user-scoped, so 0o700/0o600 keep it tidy and
// private without being surprising.
const (
	dirPerm  fs.FileMode = 0o700
	filePerm fs.FileMode = 0o600
)

// State is the persisted, on-disk memory of the plant. It is a superset of the
// pure plant.State: it carries the same remembered growth (highest stage, best
// streak) plus bookkeeping the pure state machine does not need (a schema
// version and the last-seen commit) so the file is self-describing and
// forward-compatible.
//
// The zero value is a brand-new plant with no memory, which is exactly what a
// missing or unreadable file should degrade to.
type State struct {
	// Version is the schema version of this document. It is written on save
	// and inspected on load to allow future migrations. A zero value in a
	// loaded file is treated as "legacy/unknown" and normalized forward.
	Version int `json:"version"`

	// HighestStage is the tallest stage the plant has ever reached, stored
	// by its stable string name (e.g. "leafy") rather than a raw integer so
	// the file stays readable and resilient to reordering of the Stage enum.
	HighestStage string `json:"highest_stage"`

	// BestStreak is the longest streak (in consecutive days) ever achieved.
	BestStreak int `json:"best_streak"`

	// LastCommitHash is the hash of the most recently seen commit, when
	// known. It is optional (may be empty) and is carried for future
	// features (e.g. detecting brand-new commits since the last run); the
	// pure state machine does not use it today.
	LastCommitHash string `json:"last_commit_hash,omitempty"`

	// LastCommitTime is the author timestamp of the most recently seen
	// commit, when known. It is the zero time when nothing has been seen.
	LastCommitTime time.Time `json:"last_commit_time,omitempty"`

	// WateredDates lists the calendar days (YYYY-MM-DD, in the local zone at
	// the time of watering) on which the user ran `commit-sprout water` to buy
	// a grace day. It is stored as human-readable dates rather than a bare
	// counter for three reasons: it is auditable in the JSON file, it makes
	// "already watered today" trivially idempotent, and it lets a fresh commit
	// prune waterings that predate it so grace only ever covers the *current*
	// dry spell. The slice is kept sorted and de-duplicated by normalize; only
	// the most recent MaxWateredDatesKept entries are retained so the file
	// cannot grow without bound.
	WateredDates []string `json:"watered_dates,omitempty"`
}

// MaxWateredDatesKept bounds how many watering dates are retained on disk. Only
// the most recent few ever matter (grace is capped and scoped to the current
// gap), so older entries are pruned to keep the state file small.
const MaxWateredDatesKept = 16

// DefaultState returns the state used when there is nothing on disk yet (or the
// existing file could not be read). It is a fresh plant stamped with the
// current schema version.
func DefaultState() State {
	return State{
		Version:      SchemaVersion,
		HighestStage: plant.Seed.String(),
	}
}

// Plant projects the persisted State onto the pure plant.State consumed by
// plant.Compute. Unknown/garbage stage names degrade safely to Seed, and a
// negative best streak is clamped to zero, so a hand-edited or partially
// corrupt file can never feed nonsense into the state machine.
//
// now is the reference clock and lastCommit is the author time of the most
// recent commit observed this run (zero when there are no commits). They are
// used to derive the effective watering-can grace: only waterings that fall
// after the last commit and on or before today count, capped at
// plant.MaxGraceDays, so grace always scopes to the current dry spell and can
// never be stockpiled into a permanent green streak.
func (s State) Plant(now time.Time, lastCommit time.Time) plant.State {
	best := s.BestStreak
	if best < 0 {
		best = 0
	}
	return plant.State{
		HighestStage: parseStage(s.HighestStage),
		BestStreak:   best,
		GraceDays:    s.EffectiveGrace(now, lastCommit),
	}
}

// EffectiveGrace returns how many watering-can grace days currently apply to
// the ongoing dry spell, clamped to [0, plant.MaxGraceDays].
//
// A watering counts only when its date is strictly after the last commit's
// calendar day (so committing "uses up" and invalidates earlier waterings) and
// is not in the future relative to now. When there is no commit at all, every
// recorded watering on or before today counts (there is no commit to reset
// against yet). The count is capped so at most a couple of neglected days can
// be papered over.
func (s State) EffectiveGrace(now time.Time, lastCommit time.Time) int {
	if len(s.WateredDates) == 0 {
		return 0
	}
	if now.IsZero() {
		now = time.Now()
	}
	today := dayStamp(now)

	// Waterings must post-date the last commit's day; a fresh commit wipes
	// the slate. With no commit yet, use a sentinel that lets everything
	// through.
	var floor string
	if !lastCommit.IsZero() {
		floor = dayStamp(lastCommit.In(now.Location()))
	}

	count := 0
	for _, d := range s.WateredDates {
		if d > today {
			continue // future watering (clock skew / hand edit): ignore
		}
		if floor != "" && d <= floor {
			continue // predates or coincides with the last commit day
		}
		count++
	}
	if count > plant.MaxGraceDays {
		count = plant.MaxGraceDays
	}
	return count
}

// WaterResult describes the outcome of a Water call so the CLI can report it.
type WaterResult struct {
	// Applied is true when this call recorded a new grace day. It is false
	// when watering was a no-op (already watered today, or already at the
	// grace cap), in which case Reason explains why.
	Applied bool

	// Reason is a short, human-readable explanation when Applied is false.
	Reason string

	// Grace is the effective grace-day count after this call.
	Grace int
}

// Water records a watering for now's calendar day and returns the updated State
// alongside a WaterResult describing what happened. It is deliberately
// conservative to prevent gaming the streak:
//
//   - Watering is idempotent per day: a second call on the same date is a
//     no-op (you cannot bank multiple grace days from one day of watering).
//   - Watering is capped: once effective grace is at plant.MaxGraceDays,
//     further watering is refused until a commit resets the dry spell.
//
// lastCommit is the most recent commit time (zero when none), used only to
// scope the cap check to the current gap. The returned State is normalized and
// ready to Save; the receiver is not mutated.
func (s State) Water(now time.Time, lastCommit time.Time) (State, WaterResult) {
	if now.IsZero() {
		now = time.Now()
	}
	today := dayStamp(now)

	// Already watered today? Idempotent no-op.
	for _, d := range s.WateredDates {
		if d == today {
			return normalize(s), WaterResult{
				Applied: false,
				Reason:  "already watered today",
				Grace:   s.EffectiveGrace(now, lastCommit),
			}
		}
	}

	// At the cap already? Refuse rather than silently banking a date that
	// would never take effect.
	if s.EffectiveGrace(now, lastCommit) >= plant.MaxGraceDays {
		return normalize(s), WaterResult{
			Applied: false,
			Reason:  "grace is already at the maximum",
			Grace:   plant.MaxGraceDays,
		}
	}

	out := s
	out.WateredDates = append(append([]string(nil), s.WateredDates...), today)
	out = normalize(out)
	return out, WaterResult{
		Applied: true,
		Grace:   out.EffectiveGrace(now, lastCommit),
	}
}

// dayStamp formats a time as a YYYY-MM-DD calendar-day key in its own location.
func dayStamp(t time.Time) string {
	return t.Format("2006-01-02")
}

// FromPlant folds a freshly computed result back into this persisted State,
// returning the updated value ready to Save. It advances the remembered highest
// stage and best streak monotonically (they only ever ratchet upward) and
// records the last-seen commit so the file reflects the most recent run.
//
// ps is the PlantState returned by plant.Compute for this run; lastCommitHash
// and lastCommitTime describe the most recent commit observed (hash may be
// empty when unknown, and a zero time means "no commit seen"). The receiver's
// version is normalized to the current schema.
func (s State) FromPlant(ps plant.PlantState, lastCommitHash string, lastCommitTime time.Time) State {
	out := s
	out.Version = SchemaVersion

	// Highest stage ratchets up: honor the computed peak but never regress
	// below what we already remembered.
	highest := ps.UpdatedHighestStage
	if remembered := parseStage(s.HighestStage); remembered > highest {
		highest = remembered
	}
	out.HighestStage = highest.String()

	// Best streak is monotonic: keep the larger of remembered and current.
	if ps.Streak > out.BestStreak {
		out.BestStreak = ps.Streak
	}
	if out.BestStreak < 0 {
		out.BestStreak = 0
	}

	// Record the most recent commit we have actually seen. A zero time means
	// "nothing to record", so we leave any prior value untouched in that case.
	if !lastCommitTime.IsZero() {
		out.LastCommitTime = lastCommitTime
		out.LastCommitHash = lastCommitHash

		// A fresh commit ends the current dry spell, so any watering-can
		// grace from on/before the commit day has served its purpose and is
		// pruned. This is what stops watering from being banked across
		// commits into a permanent reprieve.
		out.WateredDates = pruneWaterings(out.WateredDates, lastCommitTime)
	}

	return normalize(out)
}

// pruneWaterings drops any watering dated on or before the last commit's
// calendar day, keeping only waterings that belong to the dry spell *after*
// the commit. The commit time is compared in its own location, matching how
// EffectiveGrace scopes grace.
func pruneWaterings(dates []string, lastCommit time.Time) []string {
	if len(dates) == 0 || lastCommit.IsZero() {
		return dates
	}
	floor := dayStamp(lastCommit)
	kept := dates[:0:0]
	for _, d := range dates {
		if d > floor {
			kept = append(kept, d)
		}
	}
	return kept
}

// DefaultPath returns the XDG-aware default path to the state file:
// $XDG_CONFIG_HOME/commit-sprout/state.json when XDG_CONFIG_HOME is set to an
// absolute path, otherwise ~/.config/commit-sprout/state.json.
//
// It returns an error only when neither XDG_CONFIG_HOME nor a home directory
// can be resolved, which is extremely rare in practice.
func DefaultPath() (string, error) {
	if base := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(base) {
		return filepath.Join(base, appDir, stateFile), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("store: cannot determine config directory: %w", err)
	}
	return filepath.Join(home, ".config", appDir, stateFile), nil
}

// Load reads and decodes the state file at path. Missing files and unreadable
// or malformed files are treated as a fresh start: Load returns DefaultState
// and a nil error in those cases so a first run, a manually deleted file, or a
// corrupted file all "just work" rather than crashing the CLI.
//
// A non-nil error is only returned for genuinely unexpected I/O failures (for
// example a permission error on an existing file), which callers may choose to
// surface or ignore.
func Load(path string) (State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// No file yet: a brand-new plant.
			return DefaultState(), nil
		}
		// Existing-but-unreadable: recover to a fresh plant rather than
		// blocking the CLI, but tell the caller what happened.
		return DefaultState(), fmt.Errorf("store: reading %s: %w", path, err)
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		// Corrupt/garbled JSON: recover gracefully to defaults. This is a
		// normal, non-fatal condition (e.g. a truncated legacy file), so we
		// return no error and let the next Save heal the file.
		return DefaultState(), nil
	}

	return normalize(s), nil
}

// LoadDefault loads state from DefaultPath, applying the same graceful recovery
// semantics as Load.
func LoadDefault() (State, error) {
	path, err := DefaultPath()
	if err != nil {
		return DefaultState(), err
	}
	return Load(path)
}

// Save atomically writes s as JSON to path, creating parent directories as
// needed. It writes to a temporary file in the same directory, flushes it to
// disk, and renames it over the destination, so a reader never observes a
// partially written file and a crash mid-write cannot corrupt existing state.
func Save(path string, s State) error {
	s.Version = SchemaVersion

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return fmt.Errorf("store: creating %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("store: encoding state: %w", err)
	}
	data = append(data, '\n')

	// Temp file lives in the same directory so the final rename is atomic
	// (rename across filesystems is not).
	tmp, err := os.CreateTemp(dir, stateFile+".tmp-*")
	if err != nil {
		return fmt.Errorf("store: creating temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()

	// Best-effort cleanup: if anything below fails before the rename, remove
	// the stray temp file. After a successful rename this path no longer
	// exists and the Remove is a harmless no-op.
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("store: writing temp file: %w", err)
	}
	// Flush to stable storage before the rename so the renamed file is fully
	// durable, not just present in the page cache.
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("store: syncing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("store: closing temp file: %w", err)
	}
	if err := os.Chmod(tmpName, filePerm); err != nil {
		return fmt.Errorf("store: setting permissions: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("store: replacing %s: %w", path, err)
	}
	return nil
}

// SaveDefault atomically writes s to DefaultPath.
func SaveDefault(s State) error {
	path, err := DefaultPath()
	if err != nil {
		return err
	}
	return Save(path, s)
}

// normalize brings a freshly decoded State into a consistent, current-schema
// shape: it stamps a missing/older version forward and canonicalizes the stage
// name (so an unknown or empty value becomes "seed"). BestStreak is clamped to
// non-negative to defend against hand-edited files.
func normalize(s State) State {
	if s.Version <= 0 {
		s.Version = SchemaVersion
	}
	// Round-trip the stage through the enum so the stored name is always one
	// of the canonical values.
	s.HighestStage = parseStage(s.HighestStage).String()
	if s.BestStreak < 0 {
		s.BestStreak = 0
	}
	s.WateredDates = normalizeWaterings(s.WateredDates)
	return s
}

// normalizeWaterings de-duplicates and sorts watering dates ascending, drops
// obviously invalid entries (anything not parseable as YYYY-MM-DD), and keeps
// only the most recent MaxWateredDatesKept so a long-lived state file cannot
// accumulate unbounded history. A nil/empty input stays nil so the field is
// omitted from JSON.
func normalizeWaterings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, d := range in {
		if _, err := time.Parse("2006-01-02", d); err != nil {
			continue // skip garbage / hand-edited nonsense
		}
		if _, dup := seen[d]; dup {
			continue
		}
		seen[d] = struct{}{}
		out = append(out, d)
	}
	if len(out) == 0 {
		return nil
	}
	sort.Strings(out) // lexical sort == chronological for zero-padded ISO dates
	if len(out) > MaxWateredDatesKept {
		out = out[len(out)-MaxWateredDatesKept:]
	}
	return out
}

// parseStage maps a stored stage name back to a plant.Stage. Unknown or empty
// names resolve to Seed, matching the "fresh plant" default and ensuring a
// corrupt/hand-edited value can never crash or produce an invalid stage.
func parseStage(name string) plant.Stage {
	switch name {
	case plant.Sprout.String():
		return plant.Sprout
	case plant.Leafy.String():
		return plant.Leafy
	case plant.Tall.String():
		return plant.Tall
	case plant.Blooming.String():
		return plant.Blooming
	case plant.Seed.String():
		return plant.Seed
	default:
		return plant.Seed
	}
}
