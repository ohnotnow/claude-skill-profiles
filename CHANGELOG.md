# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.2.0] - 2026-05-18

### Added
- `csp custom` command — open the skill editor pointed directly at the current project's `.claude/settings.local.json` rather than at a named profile. Every toggle auto-saves to the project file. For projects that sit at an angle to every existing profile (a Bun project that wants a couple of extra skills, a Laravel Zero CLI with no Livewire, and so on), this avoids both hand-editing JSON and cluttering the profile list with single-use entries. On a brand-new project the editor seeds from your global `~/.claude/settings.json` and writes the file out on launch, so you end up with a real settings file whether or not you toggle anything.
- `csp promote <name>` command — lift the current project's `skillOverrides` into a new named profile under `~/.config/csp/profiles/`. Useful when an ad-hoc tweak (typically from `csp custom`) turns out to be a pattern worth reusing across projects. Refuses to overwrite an existing profile unless `--force` is supplied.

## [1.1.0] - 2026-05-18

### Added
- `csp refresh` command and `r` keybinding in the profile pane: a flipped-pane TUI for triaging skills that have appeared in `~/.claude/skills/` since a profile was last touched. New skills on the left, profiles on the right; `1`/`2`/`3`/`4` sets the state per (skill, profile), `a` then a digit bulks across every profile, and `enter` accepts the safe default (`user-invocable-only`) for any profile still missing the skill.
- `csp prune` command (with `--dry-run`) for non-interactive removal of profile entries that refer to skills no longer installed. Pairs naturally with a shell function like `rmcs <skill>` that removes a skill and tidies every profile in one go.
- Auto-prune on TUI launch: opening the TUI now silently drops profile entries for deleted skills, so the profile list always reflects what's actually installed.

### Changed
- `R` (capital) now reloads profiles from disk in the profile pane; lowercase `r` has been reassigned to open the refresh screen.

## [1.0.0] - 2026-05-17

First public release.

## [0.1.0] - 2026-05-17

Initial development snapshot.

[Unreleased]: https://github.com/ohnotnow/claude-skill-profiles/compare/v1.2.0...HEAD
[1.2.0]: https://github.com/ohnotnow/claude-skill-profiles/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/ohnotnow/claude-skill-profiles/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/ohnotnow/claude-skill-profiles/compare/v0.1.0...v1.0.0
[0.1.0]: https://github.com/ohnotnow/claude-skill-profiles/releases/tag/v0.1.0
