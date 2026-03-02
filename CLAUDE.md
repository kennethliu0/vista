# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
go build -o dist/vista .
./dist/vista           # start TUI
./dist/vista init      # generate vista.json from docker compose file
./dist/vista init path/to/compose.yaml  # use a specific file
```

No test suite, linter config, or CI pipeline exists yet.

## Architecture

Vista is a Bubble Tea TUI that manages multiple service processes and aggregates their log output. All files are in `package main`.

**Core loop:** Services stream stdout/stderr lines into a shared `chan logMsg`. The Bubble Tea model subscribes via `waitForLog()` (a blocking command that re-subscribes after each message), appends lines to the active service's log buffer, and refreshes the viewport.

**Process management** (`service.go`): Each service runs via `sh -c <cmd>` with `syscall.SysProcAttr{Setpgid: true}` so the entire process group can be killed. Stop sends SIGTERM to `-PID`, then SIGKILL after 2s as fallback.

**Navigation model** (`model.go`): No focus state. `h`/`l` navigate the sidebar list (with wrap-around), `j`/`k` always scroll the viewport. All action keys (`s`, `x`, `r`, `g`, `d`, `1-9`) work unconditionally. `b` toggles sidebar visibility. `t` toggles timestamps. `f` toggles follow mode.

**Restart** (`service.go`): `r` calls `Restart(logCh)`. If the service is Stopped/Error it logs "restarting..." and delegates to `Start`. If Running/Stopping it cancels the context, sends SIGTERM, waits synchronously on the `done` channel (SIGKILL fallback after 3s), then inlines the Start logic directly — bypassing the status guard since the old goroutine's `serviceStatusMsg{Stopped}` is already enqueued in logCh before `done` closes.

**Search** (`model.go`): `/` enters search mode using a `textinput.Model`. Queries are case-insensitive regex (falls back to literal on invalid regex). `applySearchMarkers()` adds prefix markers (`>` current, `·` other matches) and inline highlighting via `highlightLine()`. `n`/`N` cycle through `matchLines`. Search state is global — query persists across view switches, matches recompute per view.

**Filter mode** (`model.go`): `F` toggles `filterMode`. When on, `applySearchMarkers()` returns only matching lines (non-matches hidden), and `matchLines` is rewritten as a 0-based index sequence into the filtered output so `n`/`N` navigation continues to work unchanged. Status bar always shows `F: filter on/off` when a search is active.

**Follow mode** (`model.go`): `followMode` defaults to `true`. `renderTickMsg` only calls `GotoBottom()` when enabled. Scrolling up (keyboard or mouse) disables it automatically. `f` re-enables and jumps to bottom.

**Config** (`main.go`): Services are loaded from `~/.config/vista/vista.json` at startup. No hot-reload.

**Init subcommand** (`init.go`): `vista init [file]` parses a service config file (auto-discovered or explicitly provided) and writes a `vista.json` to the current directory. Adds a single `"All"` global view. Refuses to overwrite an existing `vista.json`.

- Auto-discovery priority: `Procfile` → `compose.yaml` → `compose.yml` → `docker-compose.yaml` → `docker-compose.yml`
- Format is detected from the filename: `Procfile` → procfile parser (name: command lines); `*.yaml`/`*.yml` → compose parser (`docker compose up <name>` per service)
- Explicit file path overrides auto-discovery; unrecognized extensions return a clear error

## Key Conventions

- `~` and `~/...` are supported in service `dir` and `envFile` fields and are expanded at load time via `expandHome`
- `envFile` is optional — if omitted, `serviceEnv()` auto-discovers `.env` in the service's `dir` (silently skipped if absent). Explicit `envFile` is validated at load time. Env vars are overlaid on top of `os.Environ()`.
- Commands are executed through `sh -c`, so shell syntax (pipes, loops) works
- `Start()` and `Stop()` return `nil` as a `tea.Cmd` (not from inside the closure) when the operation is a no-op, to avoid sending nil `tea.Msg` which panics Bubble Tea
- Status updates flow through two paths: `serviceStatusMsg` for start/stop events, and `logMsg` with `[vista]` prefix for process lifecycle events logged to the buffer

## Dependencies

Charmbracelet stack (v1 stable): bubbletea v1.3.10, bubbles v1.0.0, lipgloss v1.1.0. Use Context7 MCP for up-to-date API docs.

`gopkg.in/yaml.v3` — used by `init.go` for Docker Compose parsing.
