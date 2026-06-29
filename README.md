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

> Status: **early days** — see [PLAN.md](./PLAN.md) and the milestone issues.

## Why

GitHub's green squares are a chart. A chart doesn't care if you live or die. A *plant* does.
`commit-sprout` makes your recent commit activity into a small creature you keep alive — the same
dopamine as a contribution streak, but with a face (well, leaves).

## Quick idea of the interface

```bash
commit-sprout            # render the current plant for the repo you're in
commit-sprout status     # streak, last commit, days until it wilts
commit-sprout --prompt   # compact glyph for your shell prompt / tmux status
commit-sprout water      # (planned) buy a grace day before wilting
```

## Install

Not released yet. Once M6 lands there'll be a single static binary for Windows/macOS/Linux and a
`go install` path. Watch the repo.

## How it works (planned)

1. Shell out to `git log` for the current repo, count commits per day by your author email.
2. Map that activity → a growth **stage** (seed → sprout → leafy → tall → blooming) plus a
   **health** modifier (wilting when you've gone quiet).
3. Persist a little state (`~/.config/commit-sprout/state.json`) so the plant remembers its best
   days and your streak.
4. Render ASCII. No cloud. No telemetry. We never read your code — just commit counts and times.

## Tech

Go + cobra + lipgloss. Boring and fast, because this might run on every shell prompt.

## Roadmap

See [PLAN.md](./PLAN.md) for the full plan, milestones (M1–M6), and the v0.2+ backlog
(multi-repo gardens, species, pests, brag export, and more).

## License

MIT (see [LICENSE](./LICENSE) once added).

---

Part of an automated tool-lab experiment. Topic: `auto-tool-lab`.
