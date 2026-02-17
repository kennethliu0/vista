# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
go build -o bin/vista .
./bin/vista
```

No test suite, linter config, or CI pipeline exists yet.

## Architecture

Vista is a Bubble Tea TUI that manages multiple service processes and aggregates their log output. All files are in `package main`.

**Core loop:** Services stream stdout/stderr lines into a shared `chan logMsg`. The Bubble Tea model subscribes via `waitForLog()` (a blocking command that re-subscribes after each message), appends lines to the active service's log buffer, and refreshes the viewport.

**Process management** (`service.go`): Each service runs via `sh -c <cmd>` with `syscall.SysProcAttr{Setpgid: true}` so the entire process group can be killed. Stop sends SIGTERM to `-PID`, then SIGKILL after 2s as fallback.

**Navigation model** (`model.go`): No focus state. `h`/`l` navigate the sidebar list (with wrap-around), `j`/`k` always scroll the viewport. All action keys (`s`, `x`, `g`, `d`, `1-9`) work unconditionally. `b` toggles sidebar visibility.

**Config** (`main.go`): Services are loaded from `~/.config/vista/vista.json` at startup. No hot-reload.

## Key Conventions

- `~` and `~/...` are supported in service `dir` fields and are expanded at load time via `expandHome`
- Commands are executed through `sh -c`, so shell syntax (pipes, loops) works
- `Start()` and `Stop()` return `nil` as a `tea.Cmd` (not from inside the closure) when the operation is a no-op, to avoid sending nil `tea.Msg` which panics Bubble Tea
- Status updates flow through two paths: `serviceStatusMsg` for start/stop events, and `logMsg` with `[vista]` prefix for process lifecycle events logged to the buffer

## Dependencies

Charmbracelet stack (v1 stable): bubbletea v1.3.10, bubbles v1.0.0, lipgloss v1.1.0. Use Context7 MCP for up-to-date API docs.
