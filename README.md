# Vista

A macOS-native terminal UI for aggregating logs from multiple services in a monorepo. Vista starts, stops, and monitors processes while streaming their output into a unified TUI.

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Bubbles](https://github.com/charmbracelet/bubbles), and [Lip Gloss](https://github.com/charmbracelet/lipgloss).

## Features

- Sidebar with live service status indicators (running/stopped/error)
- Unified log viewport with per-service filtering
- **Global views** — interleaved logs from multiple services, merged by timestamp
- Auto-starts all services on launch
- Process group management (`setpgid` + `kill -PID`) for clean shutdowns on macOS
- Mouse wheel scrolling support
- 10,000-line log buffer per service

## Install

```
go build -o bin/vista .
```

## Configuration

Define your services in `~/.config/vista/vista.json` (or `./vista.json` for project-local config).
Service directories must be absolute paths.

```json
{
  "services": [
    {"name": "backend",  "cmd": "go run .", "dir": "/path/to/backend"},
    {"name": "frontend", "cmd": "npm start", "dir": "/path/to/frontend"},
    {"name": "worker",   "cmd": "python worker.py", "dir": "/path/to/worker"}
  ],
  "globalViews": [
    {"name": "All", "services": []},
    {"name": "Backend", "services": ["backend", "worker"]}
  ]
}
```

### Services

| Field | Description |
|-------|-------------|
| `name` | Display name in the sidebar |
| `cmd` | Shell command to run (executed via `sh -c`) |
| `dir` | Absolute path to the working directory |

### Global Views

| Field | Description |
|-------|-------------|
| `name` | Display name in the sidebar (prefixed with `⊞`) |
| `services` | Array of service names to include. An empty array enables all services. |

Global views can also be created at runtime with the `g` key.

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
| `g` | Create a new global view (all services enabled) |
| `d` | Delete selected global view |
| `1`–`9` | Toggle service on/off in the selected global view |
| `q` / `ctrl+c` | Stop all services and quit |

When a global view is selected, the sidebar splits to show a toggle panel at the bottom indicating which services are enabled or disabled.

## Project Structure

```
main.go       Entry point, config loading (services + global views)
model.go      Bubble Tea model (Init, Update, View)
service.go    Service struct, Start/Stop with process groups
messages.go   Custom message types, log channel listener, globalView type
ui.go         Lipgloss styles and layout helpers
```
