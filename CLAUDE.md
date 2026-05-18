# claude-skill-profiles (csp)

A small Go CLI and Bubble Tea TUI for managing Claude Code skill exposure profiles. Writes named profiles to `~/.config/csp/profiles/<name>.yaml` and applies them to a project's `.claude/settings.local.json` with `csp apply`.

The public-facing pitch is in `README.md`. This file is for *you* — context for working on the codebase.

## What and why

- **What:** flips entries in the `skillOverrides` block of a project's settings file according to a named profile.
- **Why:** the user has 50+ skills installed; different projects want different sets exposed; hand-editing `settings.local.json` each time was tedious enough to write a tool.
- **Spirit:** small, single-purpose, prefer simple over clever. The user will push back on over-engineering.

## Stack

- Go 1.24+
- `github.com/spf13/cobra` — CLI subcommand framework
- `github.com/charmbracelet/bubbletea` + `bubbles` + `lipgloss` — TUI
- `gopkg.in/yaml.v3` — profile file format

## Package layout

```
main.go              entry; rewrites --version to `version` then calls cmd.Execute()
cmd/                 cobra commands (one file per subcommand)
  root.go            rootCmd; running `csp` with no args opens the TUI
  list.go new.go show.go diff.go apply.go edit.go
  prune.go           non-interactive `csp prune` (with --dry-run)
  refresh.go         opens the refresh TUI; thin shim over tui.RunRefresh
  version.go         version + GitHub release-check machinery (fetchLatestRelease etc.)
  self_update.go     mirrors ait's self-update; entire implementation lives here
internal/profile/    Profile YAML schema, Store (load/save/list), SeedFromOverrides
  prune.go           Profile.Prune + PruneAll for drift handling
internal/skill/      Discover() walks ~/.claude/skills/, parses SKILL.md frontmatter
internal/settings/   ReadSkillOverrides / ApplySkillOverrides for .claude/settings.local.json
internal/tui/        Bubble Tea model (tui.go, view.go, styles.go, state.go)
  refresh.go         the refresh-screen model/view; refresh_helpers.go has the pure bits
```

## Key invariants — read before changing things

- **The four states:** `enabled`, `name-only`, `user-invocable-only`, `off`. Order matters: the TUI `1`/`2`/`3`/`4` keys index into `profile.AllStates`.
- **Replace, don't merge.** `csp apply` overwrites the whole `skillOverrides` block; other top-level keys in `settings.local.json` are preserved verbatim. We deliberately don't merge with existing local overrides — see ant ADR-001 (`ant show csp-VYQvH`) for the reasoning.
- **`~/.claude/skills/` only.** Plugin-provided skills are out of scope. Discovery never looks at `enabledPlugins`.
- **Profile YAML is explicit.** Every discovered skill gets an entry; `enabled` entries are dropped when serialising to `skillOverrides` (since enabled is Claude Code's default).
- **TUI auto-saves on every toggle.** No save key. If you add a destructive UI action, gate it behind a y/n confirm.
- **`csp new` seeds from `~/.claude/settings.json`.** Users expect a new profile to be a snapshot of their current global config, not a blank slate. The seeding helper is `profile.SeedFromOverrides`.
- **Auto-prune on TUI launch.** `tui.initialModel` calls `profile.PruneAll` against current skill discovery before anything else. Silent — no banner, no status. Removes profile entries referring to skills no longer in `~/.claude/skills/`. The user's mental model is "if I deleted a skill, csp should reflect reality" (ant ADR-002, `csp-XKtxA`). `csp prune` is the headless equivalent.
- **Refresh defaults to `user-invocable-only`.** When `csp refresh` (or `r` in the TUI) surfaces a skill missing from one or more profiles, the displayed default for the not-yet-committed pairs is `user-invocable-only`, not `enabled`. New skills are unlikely to belong in every project; this leaves them reachable via `/skill-name` without exposing them to autonomous selection. The fallback in `Profile.Get` for unmapped skills *during apply* stays `StateEnabled` — that's Claude Code's own default and the right answer for untriaged skills.
- **Dev builds skip the network.** `Version == "dev"` is the sentinel; `csp version` and `csp self-update` short-circuit before any HTTP call. Release builds get the real version/RepoURL injected via `-ldflags`.

## How to work on it

```bash
go build -o csp .       # binary (gitignored)
go test ./...           # all unit tests
./csp                   # launches the TUI
```

Tests cover the data layer (`profile`, `skill`, `settings`) and the version / self-update logic comprehensively (around 60 cases). The TUI itself is **not unit-tested** — small enough to validate by running it. If you add load-bearing behaviour to the TUI, prefer extracting it into a testable internal helper.

## Where the "why" lives

This project has an `ant` notebook at `.ant/ant.db`:

```bash
ant foundation                # vision, scope, non-goals
ant list --kind adr           # binding design decisions
ant list --kind pivot         # things we changed our minds on
```

Key entries:

- `csp-AkRXV` — foundation (what this is and isn't)
- `csp-VYQvH` — ADR-001 (replace-not-merge, `~/.claude/skills/` only, YAML, CLI surface, TUI shape)
- `csp-sYVTv` — pivot: the TUI became the *primary* edit surface, not a polish layer over `$EDITOR`. Number keys aim at a state directly; cycling was rejected as the only path.
- `csp-XKtxA` — ADR-002 (handling skill-set drift: auto-prune on TUI launch + `csp prune`, `csp refresh` for triaging new skills, the `user-invocable-only` default).

If you're about to relitigate one of these decisions, read the entry first.

There's also an `ait` history of the v1.0.0 build (`ait log`) if you want to see how the work was decomposed.

## Conventions

- **British English** in user-facing strings, comments, and docs.
- **Errors:** plain `fmt.Errorf("doing X: %w", err)`. No custom error types — we don't need them, and ait's `CLIError` machinery was deliberately not ported.
- **Comments:** explain *why*, not *what*. Single-line for invariants and gotchas; longer for non-obvious trade-offs.
- **Tests:** table-driven where the cases share shape; individual `t.Run` blocks otherwise. Use `httptest` for anything touching GitHub's API (see `cmd/self_update_test.go`).
- **No `git add`/`commit`/`push`** from inside the agent — the user has those blocked. Print the commands instead and let them run them.

## TUI keybindings (so you don't have to reverse-engineer)

Profile pane: `j/k` nav, `tab/→` switch to skills, `n` new, `a` apply, `e` `$EDITOR`, `d` delete, `r` refresh (triage new skills), `R` reload from disk, `q` quit.

Skill editor: `j/k` nav, `1/2/3/4` set + auto-advance, `tab`/`shift+tab` cycle current skill, `a` then digit bulk-sets every filtered skill, `/` filter, `s` toggle sort, `esc/←` back.

Refresh screen: `j/k` nav within focused pane, `tab/shift+tab` switch pane, `1/2/3/4` set (skill,profile) state + advance profiles, `a` then digit bulks the highlighted skill across every profile, `enter` writes `user-invocable-only` to any profile still missing the skill, `esc/q` back to main TUI (or quit if launched via `csp refresh`).

## Release process

1. Bump version by tagging — `git tag vX.Y.Z && git push origin vX.Y.Z`.
2. `.github/workflows/release.yml` builds 6 binaries (linux/darwin/windows × amd64/arm64), generates `SHA256SUMS`, publishes a GitHub release with auto-generated notes.
3. The workflow injects `Version` and `RepoURL` via `-ldflags` against `claude-skill-profiles/cmd`.

`csp version` and `csp self-update` use `https://api.github.com/repos/<RepoURL>/releases/latest`. The workflow uses `${{ github.repository }}` so forks pick up their own repo automatically.

## Out of scope (don't add without asking)

- Plugin skill management
- Merging with existing local `skillOverrides`
- A "global default" profile that auto-applies
- Multi-tool support (configuring tools beyond Claude Code)

These were considered and explicitly deferred. If a user asks for one, check whether the original scope decision still holds before implementing.
