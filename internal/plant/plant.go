// Package plant is the state machine that maps git Activity plus persisted
// State into a PlantState (stage, health, mood). It performs no I/O, making it
// the most heavily unit-tested module in the project.
//
// Stages progress seed -> sprout -> leafy -> tall -> blooming, with a health
// modifier (healthy / thirsty / wilting) that degrades the plant when commits
// have gone quiet. The whole package is pure: Compute takes an Activity, a
// remembered State, and a reference clock, and returns a deterministic
// PlantState. Same inputs always yield the same output, so it can be exercised
// entirely with table-driven tests and no live repository.
//
// All tunable numbers live together in the "Tuning thresholds" block below so
// the plant's personality can be adjusted in exactly one place.
package plant

import (
	"time"

	"github.com/rwrife/commit-sprout/internal/gitstat"
)

// ---------------------------------------------------------------------------
// Tuning thresholds
//
// Everything that decides how the plant grows and wilts is gathered here so the
// behavior is easy to reason about and tune. Nothing else in the package should
// hard-code a magic number.
// ---------------------------------------------------------------------------

const (
	// Growth is driven primarily by the current streak: consistent daily
	// commits are what make the plant climb through its stages. These are
	// the minimum streak lengths (in consecutive days) required to *reach*
	// each stage.
	//
	//	streak >= 1  -> Sprout   (you showed up today/yesterday)
	//	streak >= 3  -> Leafy
	//	streak >= 7  -> Tall
	//	streak >= 14 -> Blooming
	//
	// A streak of 0 with no commits at all is Seed.
	sproutStreak   = 1
	leafyStreak    = 3
	tallStreak     = 7
	bloomingStreak = 14

	// busyDayCommits is a "productive burst" shortcut: a lot of commits in a
	// single window, even without a long streak, is enough to nudge a bare
	// seed up to a sprout. It keeps the plant from feeling dead on the very
	// first productive day before any streak has accumulated.
	busyDayCommits = 3

	// Health is driven by how long it has been since the last commit,
	// measured in whole calendar days in the reference timezone.
	//
	//	<= thirstyAfterDays  -> Healthy
	//	<= wiltingAfterDays  -> Thirsty
	//	>  wiltingAfterDays  -> Wilting
	//
	// "Today" is 0 days idle, "yesterday" is 1, and so on.
	thirstyAfterDays = 1 // still healthy through yesterday
	wiltingAfterDays = 3 // thirsty on days 2-3, wilting from day 4+

	// MaxGraceDays caps how many watering-can grace days can ever be in
	// effect at once. Watering (`commit-sprout water`) buys the plant extra
	// idle days before it turns thirsty/wilts -- a deliberate "I'm on PTO /
	// weekend" escape hatch. The cap is what keeps watering from being spammed
	// into a permanent green streak: at most MaxGraceDays of neglect can be
	// papered over, and crucially grace only shifts *health* thresholds, never
	// the growth stage or the streak count (see applyGrace / Compute).
	MaxGraceDays = 2

	// MaxPests bounds how many cosmetic pests (aphids/weeds) the plant can
	// show at once. Messy git behavior -- reverts today -- spawns pests as a
	// gentle, purely visual nudge; the count is clamped here so a churny day
	// can never bury the plant. Pests never lower Stage, Streak, or Health;
	// they clear after a clean healthy commit day (see pestsFor).
	MaxPests = 3
)

// Stage is the plant's growth stage. Stages are ordered; higher values are
// more grown. The zero value is Seed.
type Stage int

const (
	// Seed is a plant with no activity yet -- nothing has sprouted.
	Seed Stage = iota
	// Sprout is the first green: a recent commit or a fresh streak.
	Sprout
	// Leafy is a plant with a few consecutive days of commits.
	Leafy
	// Tall is a well-established streak.
	Tall
	// Blooming is the reward stage for a long, consistent streak.
	Blooming
)

// String returns a lowercase, stable name for the stage. These names are part
// of the CLI/prompt contract (they can surface in output), so keep them stable.
func (s Stage) String() string {
	switch s {
	case Seed:
		return "seed"
	case Sprout:
		return "sprout"
	case Leafy:
		return "leafy"
	case Tall:
		return "tall"
	case Blooming:
		return "blooming"
	default:
		return "unknown"
	}
}

// ParseStage maps a stage name (as produced by Stage.String) back to a Stage.
// Unknown or empty names resolve to Seed, matching the "fresh plant" default so
// a corrupt or hand-edited value can never produce an invalid stage. It is the
// exported inverse of Stage.String, shared by persistence and display code.
func ParseStage(name string) Stage {
	switch name {
	case Sprout.String():
		return Sprout
	case Leafy.String():
		return Leafy
	case Tall.String():
		return Tall
	case Blooming.String():
		return Blooming
	default:
		return Seed
	}
}

// Health is the plant's condition modifier, driven by commit recency. The zero
// value is Healthy.
type Health int

const (
	// Healthy: committed recently (today or yesterday).
	Healthy Health = iota
	// Thirsty: a short dry spell -- a gentle nudge, not yet wilting.
	Thirsty
	// Wilting: no commits for several days; the plant is visibly drooping.
	Wilting
)

// String returns a lowercase, stable name for the health state.
func (h Health) String() string {
	switch h {
	case Healthy:
		return "healthy"
	case Thirsty:
		return "thirsty"
	case Wilting:
		return "wilting"
	default:
		return "unknown"
	}
}

// State is the remembered, persisted memory of the plant between runs. It is
// deliberately small and lives here (rather than in the store package) so that
// plant.Compute has no dependency on I/O and can be tested in isolation. The
// store package (M5) is responsible for loading/saving a value that maps onto
// this shape.
//
// The zero value is a brand-new plant that has never grown: HighestStage Seed,
// no best streak.
type State struct {
	// HighestStage is the tallest stage the plant has ever reached. It acts
	// as a soft floor so a single missed day doesn't collapse a mature plant
	// straight back to a seed; see Compute for exactly how it is applied.
	HighestStage Stage

	// BestStreak is the longest streak ever achieved. It is carried for
	// display/brag purposes and does not currently affect the computed
	// stage, but lives here so persistence is forward-compatible.
	BestStreak int

	// GraceDays is the number of watering-can grace days currently in effect
	// for the ongoing dry spell. It pushes the thirsty/wilting thresholds
	// back by that many idle days so a deliberately watered plant survives a
	// weekend or PTO without a commit. It never affects the growth Stage or
	// the Streak -- watering buys health, not fake progress -- and is clamped
	// to [0, MaxGraceDays] by Compute so a hand-edited file cannot grant an
	// unbounded reprieve. It resets to 0 once a fresh commit lands (the store
	// prunes stale waterings), so grace only ever covers the current gap.
	GraceDays int
}

// PlantState is the fully-resolved state of the plant for one render: its
// current growth Stage, its Health modifier, and a short Mood line with
// personality. It is a pure function of (Activity, State, now) and carries no
// behavior of its own.
type PlantState struct {
	// Stage is the growth stage to render.
	Stage Stage

	// Health is the condition modifier to render.
	Health Health

	// Streak is the current streak, surfaced for convenience (mirrors
	// Activity.Streak) so renderers/status output don't need the raw
	// Activity.
	Streak int

	// DaysSinceCommit is the number of whole calendar days since the last
	// commit, in the reference timezone. It is 0 when a commit landed today
	// and -1 when there are no commits at all (nothing to measure from).
	DaysSinceCommit int

	// GraceDays is the number of watering-can grace days actually in effect
	// for this render (clamped to [0, MaxGraceDays]). It is surfaced so status
	// output can report remaining grace and a corrected next-wilt date; it
	// does not influence Stage or Streak.
	GraceDays int

	// Pests is the number of cosmetic pests (aphids/weeds) currently on the
	// plant, clamped to [0, MaxPests]. Pests are spawned by messy git
	// behavior (revert commits in the window) and are purely visual + moral:
	// they never lower Stage, Streak, or Health. They clear after a clean
	// healthy commit day (a commit today with no fresh reverts), so returning
	// to a healthy cadence tidies the plant up. It is always 0 when pests are
	// disabled by the caller.
	Pests int

	// Mood is a short, flavorful one-liner describing how the plant "feels"
	// given its stage and health. Kept terse and with a little personality.
	Mood string

	// UpdatedHighestStage is the highest stage the plant has now reached,
	// i.e. max(State.HighestStage, computed live stage). Callers that
	// persist state (M5) should store this back so growth is remembered.
	UpdatedHighestStage Stage
}

// Compute maps activity plus remembered state into a PlantState, as of the
// reference time now. It is pure and deterministic: identical inputs always
// produce an identical PlantState.
//
// Growth comes from the live streak (with a small "busy day" shortcut), then is
// floored by the highest stage ever reached so a mature plant degrades in
// *health* first rather than instantly regressing to a seed on one quiet day.
// A prolonged silence (Wilting) does allow the visible stage to slip one step
// below its remembered peak, so neglect eventually shows in growth too -- but
// never below Sprout once anything has ever grown.
func Compute(act gitstat.Activity, st State, now time.Time) PlantState {
	return ComputeWith(act, st, now, DefaultHealthTuning())
}

// HealthTuning carries the idle-day thresholds that decide when a plant turns
// thirsty and then wilts. It exists so a species can bend the wilt rules (e.g.
// a drought-hardy cactus) without the pure plant package importing the species
// package, which would create an import cycle. Callers that have no species in
// hand should pass DefaultHealthTuning().
type HealthTuning struct {
	// ThirstyAfterDays is the last idle day count still considered Healthy.
	ThirstyAfterDays int
	// WiltingAfterDays is the last idle day count still considered Thirsty;
	// beyond it the plant Wilts.
	WiltingAfterDays int
}

// DefaultHealthTuning returns the shared, historical wilt thresholds used when
// a species offers no override. These match the package's original constants so
// a default plant behaves exactly as before.
func DefaultHealthTuning() HealthTuning {
	return HealthTuning{
		ThirstyAfterDays: thirstyAfterDays,
		WiltingAfterDays: wiltingAfterDays,
	}
}

// ComputeWith is Compute with an explicit HealthTuning so a species can widen
// (or tighten) how long the plant tolerates a dry spell before wilting. Growth
// (stage/streak) is unaffected -- only the health thresholds move -- so a
// species twist can never fake progress, only resilience. It remains pure and
// deterministic.
func ComputeWith(act gitstat.Activity, st State, now time.Time, tune HealthTuning) PlantState {
	days := daysSinceCommit(act, now)
	grace := clampGrace(st.GraceDays)
	health := healthForTuned(days, act.HasCommits, grace, tune)
	live := liveStage(act)

	// Remember the tallest we've ever been (peak of memory and live growth).
	highest := st.HighestStage
	if live > highest {
		highest = live
	}

	// Resolve the stage to render. Start from the live stage, then apply the
	// remembered floor so we don't yo-yo on a single missed day.
	stage := live
	if highest > stage {
		stage = flooredStage(highest, health)
	}

	return PlantState{
		Stage:               stage,
		Health:              health,
		Streak:              act.Streak,
		DaysSinceCommit:     days,
		GraceDays:           grace,
		Pests:               pestsFor(act, days),
		Mood:                moodFor(stage, health, act),
		UpdatedHighestStage: highest,
	}
}

// pestsFor maps recent messy git behavior to a bounded, cosmetic pest count.
// The only signal in v1 is revert commits in the window: each spawns a pest,
// clamped to [0, MaxPests]. Pests are gentle by design -- they never touch
// Stage, Streak, or Health.
//
// They clear on a clean healthy commit day: when the last commit landed today
// (days == 0) and that day introduced no new reverts, the plant is considered
// tidied and shows zero pests. Any revert dated today keeps its pest visible so
// a revert-heavy day still reflects, but committing normally the next day
// clears the slate.
func pestsFor(act gitstat.Activity, days int) int {
	if act.Reverts <= 0 {
		return 0
	}
	// A clean healthy commit today (no reverts among today's work) tidies the
	// plant. We approximate "today's reverts" conservatively: only when the
	// most recent commit is today do we allow clearing, and only if that day's
	// commits were revert-free. Since Reverts is a window aggregate, we treat a
	// commit-today with fewer reverts than commits as "there was clean work
	// today" and clear, matching the gentle intent.
	if days == 0 && act.TotalInWindow > act.Reverts {
		return 0
	}
	pests := act.Reverts
	if pests > MaxPests {
		pests = MaxPests
	}
	return pests
}

// liveStage computes the stage implied purely by current activity, ignoring any
// remembered peak. Growth is streak-driven, with a busy-day shortcut so the
// first productive day already sprouts something.
func liveStage(act gitstat.Activity) Stage {
	switch {
	case act.Streak >= bloomingStreak:
		return Blooming
	case act.Streak >= tallStreak:
		return Tall
	case act.Streak >= leafyStreak:
		return Leafy
	case act.Streak >= sproutStreak:
		return Sprout
	case act.HasCommits && act.TotalInWindow >= busyDayCommits:
		// A burst of commits with no established streak still counts as a
		// sprout -- you clearly did something.
		return Sprout
	default:
		return Seed
	}
}

// flooredStage decides how much of a remembered peak stage the plant keeps when
// its live growth has fallen behind. While Healthy or Thirsty the plant holds
// its remembered peak (a couple of quiet days shouldn't visibly shrink a mature
// plant). Once Wilting, it slips exactly one stage below its peak to make
// prolonged neglect visible -- but never below Sprout, so a plant that has ever
// grown never fully reverts to a bare seed.
func flooredStage(highest Stage, health Health) Stage {
	if health != Wilting {
		return highest
	}
	slipped := highest - 1
	if slipped < Sprout {
		slipped = Sprout
	}
	return slipped
}

// daysSinceCommit returns whole calendar days between the last commit and now,
// in now's timezone. It returns -1 when there are no commits (nothing to
// measure from). A commit earlier today yields 0, yesterday 1, and so on.
func daysSinceCommit(act gitstat.Activity, now time.Time) int {
	if !act.HasCommits || act.LastCommit.IsZero() {
		return -1
	}
	loc := now.Location()
	last := dayStart(act.LastCommit.In(loc))
	today := dayStart(now)
	d := int(today.Sub(last).Hours() / 24)
	if d < 0 {
		// A commit timestamped slightly in the future (clock skew) is
		// treated as "today" rather than a negative age.
		d = 0
	}
	return d
}

// dayStart truncates a time to midnight in its own location, giving a stable
// calendar-day anchor for day-difference math.
func dayStart(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// healthForTuned is healthFor with explicit thresholds supplied by the caller
// (typically from a species). See healthFor for the shared-default behavior.
func healthForTuned(days int, hasCommits bool, grace int, tune HealthTuning) Health {
	if !hasCommits || days < 0 {
		return Healthy
	}
	effective := days - grace
	if effective < 0 {
		effective = 0
	}
	switch {
	case effective <= tune.ThirstyAfterDays:
		return Healthy
	case effective <= tune.WiltingAfterDays:
		return Thirsty
	default:
		return Wilting
	}
}

// healthFor maps days-since-last-commit to a Health. With no commits at all the
// plant is a fresh Seed and reported Healthy (there is nothing to wilt yet).
//
// grace is the number of watering-can days in effect (already clamped): it
// shifts the idle count back so a watered plant tolerates that many extra quiet
// days before drying out. Grace only moves the health thresholds; the caller
// keeps Stage and Streak untouched, so watering can never fake growth.
func healthFor(days int, hasCommits bool, grace int) Health {
	return healthForTuned(days, hasCommits, grace, DefaultHealthTuning())
}

// clampGrace bounds a raw grace-day count into the supported [0, MaxGraceDays]
// range so neither a negative nor an over-large (hand-edited) value can distort
// the health calculation.
func clampGrace(g int) int {
	if g < 0 {
		return 0
	}
	if g > MaxGraceDays {
		return MaxGraceDays
	}
	return g
}

// moodFor returns a short flavor line for the given stage/health, with a little
// personality. Health takes priority for the "problem" states (thirsty/wilting)
// so the message nudges you to commit; otherwise the message celebrates the
// current growth stage.
func moodFor(stage Stage, health Health, act gitstat.Activity) string {
	switch health {
	case Wilting:
		return "Parched and drooping. A commit today would really help."
	case Thirsty:
		return "Getting a little dry -- a commit soon keeps it perky."
	}

	// Healthy: celebrate the stage.
	switch stage {
	case Seed:
		return "Just a seed in the soil. Commit something to make it sprout."
	case Sprout:
		return "A fresh little sprout. Keep the streak going!"
	case Leafy:
		return "Leafing out nicely -- a few solid days in a row."
	case Tall:
		return "Standing tall on a healthy streak. Impressive cadence."
	case Blooming:
		return "In full bloom! That is a serious commit streak. 🌸"
	default:
		return "Growing along."
	}
}
