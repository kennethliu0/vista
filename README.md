# Vista

A macOS-native terminal UI for aggregating logs from multiple services in a monorepo. Vista starts, stops, and monitors processes while streaming their output into a unified TUI.

![Vista Screenshot](https://github.com/user-attachments/assets/feff1d36-6bcc-4101-ad0b-5378f001ddc0)

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Bubbles](https://github.com/charmbracelet/bubbles), and [Lip Gloss](https://github.com/charmbracelet/lipgloss).

## Features

- Sidebar with live service status indicators (running/stopped/error)
- Unified log viewport with per-service filtering
- **Global views** — interleaved logs from multiple services, merged by timestamp
- **Log search** — `/` to search with regex or plain text, with inline match highlighting and `n`/`N` navigation
- **Timestamps** — `t` toggles millisecond-precision timestamps (`HH:MM:SS.mmm`) on each log line
- **Follow mode** — auto-scrolls with new logs; scrolling up pauses, `f` resumes
- Auto-starts all services on launch
- Process group management (`setpgid` + `kill -PID`) for clean shutdowns on macOS
- Mouse wheel scrolling support
- 10,000-line log buffer per service

## Install

```
go build -o bin/vista .
```

## Quick Start

If you have a Docker Compose project, generate a `vista.json` automatically:

```
vista init
```

Vista searches the current directory for `compose.yaml`, `compose.yml`, `docker-compose.yaml`, or `docker-compose.yml` and writes a `vista.json` that runs each service individually via `docker compose up <name>`. You can also point it at a specific file:

```
vista init path/to/custom-compose.yaml
```

## Configuration

Define your services in `./vista.json` for each project (or `~/.config/vista/vista.json` for global config).

```json
{
  "services": [
    {"name": "backend",  "cmd": "go run .", "dir": "~/projects/backend"},
    {"name": "frontend", "cmd": "npm start", "dir": "~/projects/frontend"},
    {"name": "worker",   "cmd": "python worker.py", "dir": "~/projects/worker"}
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
| `dir` | Working directory. Supports `~` expansion. |

### Global Views

| Field | Description |
|-------|-------------|
| `name` | Display name in the sidebar (prefixed with `⊞`) |
| `services` | Array of service names to include. An empty array enables all services. |

Global views can also be created at runtime with the `g` key.

## Usage

```
./bin/vista          # start the TUI
./bin/vista init     # generate vista.json from a compose file
```

## Key Bindings

| Key | Action |
|-----|--------|
| `h` / `l` | Switch to previous / next service or global view |
| `j` / `k` | Scroll logs up / down |
| `s` | Start selected service |
| `x` | Stop selected service |
| `g` | Create a new global view (all services enabled) |
| `d` | Delete selected global view |
| `1`–`9` | Toggle service on/off in the selected global view |
| `b` | Hide / show the sidebar |
| `t` | Toggle timestamps |
| `f` | Toggle follow mode (auto-scroll) |
| `/` | Enter search mode |
| `n` / `N` | Jump to next / previous match |
| `esc` | Clear search and return to normal mode |
| `q` / `ctrl+c` | Stop all services and quit |

When a global view is selected, the sidebar splits to show a toggle panel at the bottom indicating which services are enabled or disabled. Use `1`–`9` to toggle individual services in the view.

### Search

Press `/` to open the search bar at the bottom of the screen. Queries are treated as **case-insensitive regular expressions** — plain text works too. While typing:

- Matching lines get a `·` prefix; the current match gets a `>` prefix
- Matched text is highlighted inline (yellow for the current match, orange for others)
- The status bar shows `n/N matches · n/N: navigate · esc: clear`

Press `enter` to commit the query and use `n`/`N` to jump between matches. Press `esc` to clear the search entirely. If the regex is invalid, Vista falls back to literal substring matching and shows the error in the status bar.

## Project Structure

```
main.go       Entry point, config loading (services + global views), subcommand dispatch
init.go       `vista init` — generates vista.json from a Docker Compose file
model.go      Bubble Tea model (Init, Update, View)
service.go    Service struct, Start/Stop with process groups
messages.go   Custom message types, log channel listener, globalView type
ui.go         Lipgloss styles and layout helpers
```
