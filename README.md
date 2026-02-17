# Vista

A macOS-native terminal UI for aggregating logs from multiple services in a monorepo. Vista starts, stops, and monitors processes while streaming their output into a unified TUI.

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Bubbles](https://github.com/charmbracelet/bubbles), and [Lip Gloss](https://github.com/charmbracelet/lipgloss).

## Features

- Sidebar with live service status indicators (running/stopped/error)
- Unified log viewport with per-service filtering
- Auto-starts all services on launch
- Process group management (`setpgid` + `kill -PID`) for clean shutdowns on macOS
- Mouse wheel scrolling support
- 10,000-line log buffer per service

## Install

```
go build -o bin/vista .
```

## Configuration

Define your services in `~/.config/vista/vista.json`:
Note that service directories must be absolute paths.

```json
{
  "services": [
    {"name": "backend",  "cmd": "cmd-to-start", "dir": "/path/to/backend"},
    {"name": "frontend", "cmd": "cmd-to-end",  "dir": "/path/to/frontend"}
  ]
}
```

| Field | Description |
|-------|-------------|
| `name` | Display name in the sidebar |
| `cmd` | Shell command to run (executed via `sh -c`) |
| `dir` | Absolute path to the working directory |

## Usage

```
./bin/vista
```

## Key Bindings

| Key | Action |
|-----|--------|
| `j` / `k` | Navigate services (sidebar) or scroll logs (viewport) |
| `tab` | Toggle focus between sidebar and viewport |
| `s` | Start selected service |
| `x` | Stop selected service |
| `q` / `ctrl+c` | Stop all services and quit |

## Project Structure

```
main.go       Entry point, service definitions
model.go      Bubble Tea model (Init, Update, View)
service.go    Service struct, Start/Stop with process groups
messages.go   Custom message types, log channel listener
ui.go         Lipgloss styles and layout helpers
```
