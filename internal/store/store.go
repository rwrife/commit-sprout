// Package store loads and saves the small JSON state file that gives the plant
// memory between runs (highest stage reached, current streak, last-seen
// commit). It uses atomic writes and sane defaults, and defaults its location
// to ~/.config/commit-sprout/state.json.
//
// Implementation lands in M5 (Persistent state + streaks). This stub exists so
// the package directory and intent are established during M1 scaffolding.
package store
