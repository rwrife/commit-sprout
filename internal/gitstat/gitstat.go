// Package gitstat reads commit activity from a git repository and normalizes
// it into an Activity struct (commits per day, last commit time, current
// streak). It works by shelling out to `git log` and parsing the output, which
// keeps the dependency surface small and avoids heavy git bindings.
//
// Implementation lands in M2 (Read git activity). This stub exists so the
// package directory and intent are established during M1 scaffolding.
package gitstat
