# commit-sprout 🌱

> A terminal pet that grows an ASCII plant from your **real git commits**.
> Ship code → it sprouts a leaf. Keep a streak → it blooms. Ghost your repo → it wilts.

```
        ,&&&.
       .&&&&&&.        commit-sprout
      &&&& '&&&&       stage:  leafy 🌿
     '&&&&. &&&'       streak: 6 days
       '&&&&&'         last commit: 4h ago
         |||
        =====
```

`commit-sprout` reads your git history and turns "did I actually ship anything?" into something
you can *see*. It's local-first, single-binary, and has opinions about your commit cadence.

> Status: **v0.1** — the plant renders, reacts to your commits, and drops into
> your prompt. See [PLAN.md](./PLAN.md) and the milestone issues for what's next.

## Why

GitHub's green squares are a chart. A chart doesn't care if you live or die. A *plant* does.
`commit-sprout` makes your recent commit activity into a small creature you keep alive — the same
dopamine as a contribution streak, but with a face (well, leaves).

## Quick idea of the interface

```bash
commit-sprout            # render the current plant for the repo you're in
commit-sprout status     # streak, last commit, days until it wilts
commit-sprout brag       # one-line, standup-ready recap of recent growth
commit-sprout prompt     # compact glyph for your shell prompt / tmux status
commit-sprout --prompt   # same thing, as a flag (handy inside a prompt string)
commit-sprout water      # buy a grace day before wilting (weekends / PTO)
commit-sprout watch      # live full-screen view; the plant gently sways
commit-sprout garden     # a row of plants, one per repo (a windowsill)
commit-sprout --species cactus   # pick your plant's species (remembered as default)
```

`status` prints a plain, script-friendly summary:

```text
stage:   leafy (healthy)
streak:  6 days (best: 11 days)
last:    today (2026-07-04)
health:  healthy — 2 days of cushion before it gets thirsty
```

After watering, `status` also reports the grace in effect and the projected
wilt date:

```text
stage:   leafy (healthy)
streak:  6 days (best: 11 days)
last:    2 days ago (2026-07-03)
grace:   1 day (watered)
health:  healthy — commit today to stay ahead of thirst
wilts:   2026-07-08
```

### Brag line (standups)

Want a single, copy-pasteable line for your standup or Slack? `brag` recaps how
the plant grew over the recent window (default 7 days), driven entirely by real
commit data — no new state, just the shared read pipeline:

```bash
commit-sprout brag
```

```text
This week your plant grew sprout→leafy on a 5-day streak (14 commits in 7 days). 🌱
```

It phrases itself honestly: a plant that has gone quiet nudges you to revive it
rather than crowing about growth, and a brand-new repo says there's nothing to
brag about yet. The output is plain (no color) and single-line, so it drops
straight into a git `post-commit` hook for an ambient nudge:

```bash
# .git/hooks/post-commit
#!/bin/sh
commit-sprout brag
```

or into a standup helper:

```bash
echo "Standup: $(commit-sprout brag)"
```

### Watering can (grace days)

Heading into a weekend or PTO and don't want your plant to droop? Water it:

```bash
commit-sprout water
```

Each watering buys **one extra idle day** before the plant turns thirsty and,
later, wilts. It is deliberately hard to abuse, so it nudges *health* without
ever faking *progress*:

- **One per day** — watering twice on the same day is a no-op.
- **Capped** — at most two grace days can be in effect at once.
- **Self-resetting** — a fresh commit clears any grace, so you can't bank
  waterings across commits into a permanent green streak.
- **Health only** — grace never changes your growth stage or streak count; it
  just keeps a watered plant perky through a planned break.

### Watch mode (windowsill)

```bash
commit-sprout watch
commit-sprout watch --interval 2s   # re-read git activity more often
```

`watch` opens a minimal full-screen view and lets the plant live: it **sways
gently** and **re-reads your git activity on an interval** (default every 5s),
so a fresh commit sprouts without you restarting anything. Press **q** or
**Ctrl-C** to exit — the terminal (alternate screen, cursor, raw mode) is always
restored on the way out. It stays cheap: frames are throttled and only redraw on
change, so a still plant costs almost nothing. `watch` is read-only (it never
persists state) and refuses to start when stdout is not a TTY.

`prompt` prints a single glyph + stage (e.g. `ل leafy`). It is built for the
hot path: **read-only by default** (a prompt renders on every command, so it
skips the state write unless you pass `--save`) and it **fails silently outside
a git repo** — it prints nothing and exits `0`, so it can never break your
shell. Drop it in and forget it.

### Multi-repo garden (windowsill)

Most people ship across more than one repo. `commit-sprout garden` renders a
row of small plants — one per repository — so you can eyeball your cadence
everywhere at once:

```bash
commit-sprout garden ~/code/foo ~/code/bar   # explicit repos, side by side
commit-sprout garden --scan ~/code           # scan a parent dir one level deep
commit-sprout garden                         # render your saved garden
```

Example:

```text
foo                  bar
   \|/                  ,
    |                  /|
   _|_                ' |
  (   )                _|_
   '-'               (   )
seed/healthy          '-'
no commits yet     leafy/healthy
                   today (2026-07-06)
```

Save a set of repos so plain `garden` just works, and manage it over time:

```bash
commit-sprout garden --add ~/code/foo ~/code/bar   # remember these repos
commit-sprout garden --add                         # remember the current repo
commit-sprout garden --remove ~/code/bar           # forget one
commit-sprout garden --list                        # show the saved set
```

Details:

- **Repo resolution order:** explicit path args → `--scan <dir>` → your saved
  set. Whichever is present first wins.
- **Per-repo identity:** each repo is scored against *its own*
  `git config user.email` by default, so a garden of repos you committed under
  different addresses still counts correctly. Pass `--author <email>` to score
  every repo against one address instead.
- **Read-only:** the garden is a glance, not a writer — it never changes any
  repo's remembered plant state. The saved *repo list* lives in the same state
  file as everything else (`garden_repos`).
- **Layout:** plants wrap onto multiple rows to fit your terminal width
  (honoring `$COLUMNS`), and color / `--no-color` / `NO_COLOR` behave exactly
  like the single-plant view.

### Species

Pick what kind of plant you're growing. Each species has its own full art set
(across every growth stage and wilt state), and one has a gameplay twist.

```bash
commit-sprout --species fern        # the default: upright and leafy
commit-sprout --species cactus      # blocky and spiky — and drought-hardy
commit-sprout --species bonsai      # a gnarled, sculptural little tree
commit-sprout --species sunflower   # a tall stalk that ends in a big bloom
```

- **Remembered:** the species you choose is saved as your default (in the same
  state file as everything else, under `species`), so later flag-less runs keep
  rendering it. Pass `--species` again anytime to switch.
- **Cactus twist:** a cactus tolerates much longer commit gaps before it wilts
  (roughly a week and a half of silence vs. a few days for the leafier species).
  It only bends the *health* thresholds — growth and streak still come from real
  commits, so a cactus can't fake progress, just survive a drought.

## Install

### Prebuilt binary (recommended)

Grab a static, single-file binary for your OS/arch from the
[latest release](https://github.com/rwrife/commit-sprout/releases/latest) —
builds are published for Linux, macOS, and Windows (amd64 + arm64), with
checksums. No runtime to install. Unpack it and drop `commit-sprout` on your
`PATH`, e.g.:

```bash
# Linux/macOS example (adjust the asset name to your OS/arch)
curl -sSL -o commit-sprout.tar.gz \
  https://github.com/rwrife/commit-sprout/releases/latest/download/commit-sprout_<version>_linux_amd64.tar.gz
tar -xzf commit-sprout.tar.gz commit-sprout
install -m 0755 commit-sprout ~/.local/bin/   # or anywhere on your PATH
commit-sprout --version
```

### From source

With Go 1.23+ installed:

```bash
# Run straight from a clone
git clone https://github.com/rwrife/commit-sprout
cd commit-sprout
go run .            # renders the live plant for the repo you're in (add --no-color for plain ASCII)

# Or install the binary onto your PATH
go install github.com/rwrife/commit-sprout@latest
commit-sprout      # same thing
commit-sprout --version
```

## Put it in your shell prompt

`commit-sprout prompt` emits one line (`glyph stage`) and is safe to call on
every prompt render — it does no disk writes by default and stays silent outside
a git repo. A few ways to wire it up:

**starship** (`~/.config/starship.toml`) — add a custom module:

```toml
[custom.commit_sprout]
command = "commit-sprout prompt"
when = "git rev-parse --is-inside-work-tree"
shell = ["sh", "--norc"]
format = "[$output]($style) "
style = "green"
```

**tmux** (`~/.tmux.conf`) — show it in the status line (polled, so keep the
interval sane):

```tmux
set -g status-interval 60
set -g status-right "#(cd #{pane_current_path} && commit-sprout prompt) | %H:%M"
```

**bash** (`~/.bashrc`) — prepend the glyph to your prompt:

```bash
__commit_sprout() { commit-sprout prompt 2>/dev/null; }
PROMPT_COMMAND='PS1_SPROUT=$(__commit_sprout)'
PS1='${PS1_SPROUT:+$PS1_SPROUT }\u@\h:\w\$ '
```

**zsh** (`~/.zshrc`) — same idea with a precmd hook:

```zsh
autoload -Uz add-zsh-hook
__commit_sprout() { psvar[1]="$(commit-sprout prompt 2>/dev/null)"; }
add-zsh-hook precmd __commit_sprout
setopt prompt_subst
PROMPT='%(1V.%1v .)%n@%m:%~%# '
```

All of these degrade cleanly: no repo, no glyph, no error.

## How it works (planned)

1. Shell out to `git log` for the current repo, count commits per day by your author email.
   _(Implemented — M2. The reader lives in `internal/gitstat` and normalizes history into an
   `Activity` value: commits-per-day, last commit, current streak, and total in a look-back
   window. You can peek at it today with the hidden `commit-sprout --activity` debug flag.)_
2. Map that activity → a growth **stage** (seed → sprout → leafy → tall → blooming) plus a
   **health** modifier (wilting when you've gone quiet).
   _(Implemented — M3. The state machine lives in `internal/plant`: `plant.Compute(activity,
   state, now)` is a pure, deterministic function that resolves the stage (streak-driven, with a
   busy-day shortcut), a health modifier (healthy → thirsty → wilting by commit recency), and a
   short mood line. Remembered peak growth floors the stage so one quiet day dents health before
   it shrinks the plant. Thresholds all live in one tunable block.)_
3. Persist a little state (`~/.config/commit-sprout/state.json`) so the plant remembers its best
   days and your streak.
   _(Implemented — M5. `internal/store` loads/saves an XDG-aware, versioned JSON file
   (`$XDG_CONFIG_HOME/commit-sprout/state.json`, falling back to `~/.config/...`) that remembers
   the highest stage ever reached, your best streak, and the last-seen commit. Writes are atomic
   (temp file + rename) so a crash never corrupts it, and a missing or garbled file heals to sane
   defaults instead of erroring. State is loaded before compute and saved after render; pass
   `--no-save` for a read-only run that leaves your plant's memory untouched.)_
4. Render ASCII. No cloud. No telemetry. We never read your code — just commit counts and times.
   _(Implemented — M4. `internal/render` draws real ASCII art for every stage × health combo
   (the plant visibly droops as it dries out), plus a caption block with stage, streak, last
   commit, and a days-until-wilt nudge. Color is via `lipgloss`; the plain-ASCII path is used
   automatically for `--no-color`, the `NO_COLOR` env var, and any non-TTY/piped output, so it
   stays pipe-safe. `commit-sprout` renders the live plant for your repo instead of a static
   seedling, and (as of M5) remembers its best growth across runs.)_

## Tech

Go + cobra + lipgloss. Boring and fast, because this might run on every shell prompt.

## Roadmap

See [PLAN.md](./PLAN.md) for the full plan, milestones (M1–M6), and the v0.2+ backlog
(species, pests, brag export, and more).

**Shipped since v0.1 (v0.2 backlog):**

- **Watering can** (`commit-sprout water`) — buy bounded grace days for planned breaks.
- **Multi-repo garden** (`commit-sprout garden`) — a windowsill of plants, one per repo,
  with a saveable repo set and per-repo author identity.
- **Species selection** (`commit-sprout --species`) — fern / cactus / bonsai / sunflower,
  each with its own art set; the cactus survives droughts longer as a gameplay twist.
- **Animated watch mode** (`commit-sprout watch`) — a live full-screen view where the
  plant gently sways and re-reads git activity on an interval.

## License

MIT — see [LICENSE](./LICENSE).

---

Part of an automated tool-lab experiment. Topic: `auto-tool-lab`.
