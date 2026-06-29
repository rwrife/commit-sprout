# commit-sprout 🌱

## 1. Pitch

`commit-sprout` is a terminal pet that turns your real git activity into a living ASCII plant.
Commit today and it sprouts a fresh leaf; keep a streak and it grows tall and blooms; ghost your
repo for a few days and it visibly wilts. It's a tiny, local-first nudge that makes "did I ship
anything today?" something you can *see* on a windowsill in your shell.

## 2. Trend inspiration

Scanned what's hot right now, primarily via the GithubAwesome YouTube channel feed plus
r/commandline / awesome-cli roundups:

- **Desktop pets & ASCII toys are back.** Trending titles like `ASCII-Aquarium` and `planttalk`
  (GitHub Trending Weekly #34 / Today feeds on https://www.youtube.com/@GithubAwesome) show a
  clear appetite for small, delightful, animated terminal companions.
- **Personal "brag / streak" logging.** Trending `brag`, `recall`, and `superlog` point at devs
  wanting lightweight, low-friction ways to track that they actually did things.
- **The "modern CLI/TUI renaissance."** Multiple 2026 roundups (e.g.
  https://1337skills.com/blog/2026-03-09-terminal-renaissance-modern-tui-tools-reshaping-developer-workflows/
  and https://github.com/toolleeo/awesome-cli-apps-in-a-csv) confirm developers are happily
  living in the terminal again and reward tools with personality.
- **GitHub contribution-graph culture.** The green-squares streak obsession is well established;
  nobody has turned it into a *creature you have to keep alive*.

`commit-sprout` sits exactly at the intersection: an ASCII desktop-pet toy (fun) that's wired to
real commit data (useful), with brag/streak energy baked in.

## 3. Why it's different

- **vs. `ASCII-Aquarium` / generic desktop pets:** those animate for ambiance only.
  `commit-sprout`'s state is *driven by your actual work* — the plant is a readout, not decor.
- **vs. GitHub streak widgets / `git-stats` / contribution-graph art:** those render history as a
  chart. We render it as a *life you're responsible for* — growth, wilt, bloom, and (eventually)
  death, which is a stronger behavioral nudge than a flat grid.
- **vs. our own `yak-tracker` / `ship-log`:** those are *narrative logs* you read back.
  `commit-sprout` is an *ambient creature* you glance at — no reading, no journaling, just a vibe.
- **vs. `idle-hands` (break coach):** that fills wait time with micro-tasks; this is a persistent
  companion tied to long-term commit cadence, not idle moments.
- **vs. habit-tracker apps (streaks, etc.):** generic habit apps don't understand git.
  `commit-sprout` reads commits directly — zero manual check-ins, no lying to your habit tracker.

As far as I know, a commit-fed ASCII plant pet with wilt/bloom states is a fresh combination,
though "plant grows as you work" toys and GitHub streak art both exist as separate ideas.

## 4. MVP scope (v0.1)

The smallest useful thing:

- `commit-sprout` (no args) renders the current plant as ASCII to stdout.
- Reads commit activity from the git repo in the current directory (last N days), counting commits
  by the configured author (default: `git config user.email`).
- Maps activity → growth **stage** (e.g. seed → sprout → leafy → tall → blooming) and a
  **health** modifier (wilting if no commits in the last X days).
- Persists state in a small JSON file (e.g. `~/.config/commit-sprout/state.json`) so the plant
  has memory between runs (highest stage reached, current streak, last-seen commit).
- A `--prompt` one-line mode that emits a compact glyph + stage suitable for a shell prompt /
  starship / tmux status line.
- A `commit-sprout water` / `commit-sprout status` subcommand to show streak, last commit, and
  days-until-wilt.

That's it: one binary, one config dir, renders a plant that reflects your last week of commits.

## 5. Tech stack

- **Go** — single static binary, trivial cross-platform (Windows/macOS/Linux), fast cold start
  (important: this may run on every shell prompt). No runtime to install for users.
- **Standard library + `os/exec` to shell out to `git`** — avoids heavy git-binding deps; `git`
  is already present wherever this is useful. Fall back to `go-git` later only if needed.
- **`lipgloss` (charmbracelet)** for optional color/styling — battle-tested, plays nice with
  no-color terminals; degrade gracefully to plain ASCII.
- **`cobra`** for subcommands — boring, standard, good help output.
- **Plain JSON** for state — human-readable, easy to debug, zero schema ceremony.

Rationale: pick boring + fast. The one hard constraint is prompt-mode latency, which Go nails.

## 6. Architecture

Short overview — keep modules tiny and pure where possible:

- `cmd/` — cobra commands (`root`, `status`, `prompt`, `water`, `version`).
- `internal/gitstat/` — runs `git log`, returns a normalized `Activity` struct (commits/day,
  last commit time, streak length). Pure parsing, unit-testable with fixture output.
- `internal/plant/` — the state machine: `Activity` + persisted `State` → `PlantState`
  (stage, health, mood). No I/O. This is the brain and the most-tested module.
- `internal/render/` — `PlantState` → ASCII frames (and prompt glyph). Art lives here as data.
- `internal/store/` — load/save JSON state with safe defaults and atomic writes.
- `main.go` — wiring only.

Data flow: `git log` → `gitstat.Activity` → merged with `store.State` → `plant.Compute()` →
`render.Frame()` → stdout, then `store.Save()`.

## 7. Milestones

1. **M1 — Scaffold + hello-world.** Go module, cobra root command, CI, `commit-sprout` prints a
   single hard-coded ASCII seedling. Ships as a runnable binary.
2. **M2 — Read git activity.** `internal/gitstat` parses `git log` into an `Activity` struct
   (commit counts per day, last commit, current streak) with tests against fixtures.
3. **M3 — Plant state machine.** `internal/plant` maps activity → stage + health (seed → sprout
   → leafy → tall → bloom; wilting when idle). Pure, fully unit-tested.
4. **M4 — ASCII art + render.** `internal/render` draws each stage/health combo as proper ASCII
   art; `--no-color` and color (lipgloss) paths both work.
5. **M5 — Persistent state + streaks.** `internal/store` persists highest stage, streak, and
   last-seen commit to `~/.config/commit-sprout/state.json` (atomic writes, sane defaults).
6. **M6 — Prompt mode + `status` + docs.** `--prompt` compact glyph for shell/tmux, polished
   `status` output, README with install + starship/tmux snippets, and a release build.

## 8. Backlog / future features (v0.2+)

1. **Watering can grace period** — `commit-sprout water` buys the plant one extra day before it
   wilts (a deliberate "I'm on PTO" escape hatch, used sparingly).
2. **Multi-repo garden** — aggregate commits across several repos into one bigger plant, or a
   row of plants (a windowsill).
3. **Species selection** — fern, bonsai, cactus (cactus survives droughts longer), sunflower,
   mushroom; pick your vibe.
4. **Animated mode** — `commit-sprout watch` gently sways/grows in a live TUI loop.
5. **Bloom on milestones** — flowers triggered by tags/releases or a big commit day.
6. **Pests & weeds** — reverts, force-pushes, or huge red-diff days spawn aphids you must clear.
7. **Seasons / themes** — autumn palette, snow, holiday skins by date.
8. **GitHub/GitLab API source** — feed the plant from contribution data instead of a local repo
   (so it grows from *all* your work, not just the current checkout).
9. **Daily snapshot / brag export** — `commit-sprout brag` emits a tiny "this week your plant
   grew from sprout→leafy (14 commits)" line for standups.
10. **Shareable plant card** — export current plant as an SVG/PNG for a README badge.
11. **Team garden / leaderboard** — opt-in shared garden where a team's plants grow together.
12. **Notifications** — optional nudge when the plant is about to wilt (cron/`atuin`-style hook).

## 9. Out of scope

- ❌ A full GUI / Electron app. This is terminal-first, forever.
- ❌ A cloud service, accounts, or any server. State is local JSON. No telemetry.
- ❌ Real-time daemon that watches git in the background — runs on demand / per prompt only.
- ❌ Gamified social pressure / streak-shaming notifications by default (opt-in only, and gentle).
- ❌ Reading commit *content* or sending code anywhere. We count commits and timestamps, period.
- ❌ Becoming a productivity-analytics suite. It's a plant, not a dashboard.
