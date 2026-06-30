// Package plant is the state machine that maps git Activity plus persisted
// State into a PlantState (stage, health, mood). It performs no I/O, making it
// the most heavily unit-tested module in the project.
//
// Stages progress seed -> sprout -> leafy -> tall -> blooming, with a health
// modifier that wilts the plant when commits have gone quiet.
//
// Implementation lands in M3 (Plant state machine). This stub exists so the
// package directory and intent are established during M1 scaffolding.
package plant
