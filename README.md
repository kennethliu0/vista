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

**Homebrew (recommended):**

```
brew install kennethliu0/tap/vista
```

**Download binary:**

Download the latest binary from the [releases page](https://github.com/kennethliu0/vista/releases), then:

```
chmod +x vista
sudo mv vista /usr/local/bin/vista
```

**From source** (requires Go 1.26+):

```
git clone https://github.com/kennethliu0/vista.git
cd vista
go build -o dist/vista .
sudo mv dist/vista /usr/local/bin/vista
```

> **macOS note:** If you download a pre-built binary instead of building from source, macOS may block it due to Gatekeeper quarantine. Vista's install script removes the quarantine attribute automatically (`xattr -d com.apple.quarantine`). If you install manually, run that command yourself or go to System Settings → Privacy & Security to allow it.

## Quick Start

Generate a `vista.json` from an existing service config:

```
vista init
```

Vista searches the current directory in priority order and uses the first match:

| File | Format |
|------|--------|
| `Procfile` | Foreman/Overmind — uses each entry's command directly |
| `compose.yaml` / `compose.yml` | Docker Compose — runs each service via `docker compose up <name>` |
| `docker-compose.yaml` / `docker-compose.yml` | Docker Compose — same as above |

You can also point it at a specific file — the format is detected from the filename:

```
vista init path/to/Procfile
vista init path/to/custom-compose.yaml
```

If both a `Procfile` and a compose file exist, auto-discovery picks the `Procfile`. Pass the compose file explicitly to use it instead.

## Configuration

Define your services in `./vista.json` for each project (or `~/.config/vista/vista.json` for global config).

```json
{
  "profiles": [
    {
      "name": "dev",
      "default": true,
      "services": [
        {"name": "backend",  "cmd": "go run .", "dir": "~/projects/backend"},
        {"name": "frontend", "cmd": "npm start", "dir": "~/projects/frontend"},
        {"name": "worker",   "cmd": "python worker.py", "dir": "~/projects/worker"}
      ],
      "globalViews": [
        {"name": "All", "services": []},
        {"name": "Backend", "services": ["backend", "worker"]}
      ]
    },
    {
      "name": "staging",
      "services": [
        {"name": "backend",  "cmd": "./run-staging.sh", "dir": "~/projects/backend"}
      ],
      "globalViews": [
        {"name": "All", "services": []}
      ]
    }
  ]
}
```

### Profiles

A `vista.json` can contain multiple named profiles — useful for switching between dev, staging, and other environments without maintaining separate config files.

| Field | Description |
|-------|-------------|
| `name` | Profile identifier used on the CLI |
| `default` | If `true`, this profile is used when no name is given. If no profile has `"default": true`, the first profile is used. |
| `services` | Services for this profile |
| `globalViews` | Global views for this profile |

### Services

| Field | Description |
|-------|-------------|
| `name` | Display name in the sidebar |
| `cmd` | Shell command to run (executed via `sh -c`) |
| `dir` | Working directory. Supports `~` expansion. |
| `envFile` | Path to a `.env` file to load (optional). If omitted, Vista auto-loads `.env` from `dir` if one exists. |

### Global Views

| Field | Description |
|-------|-------------|
| `name` | Display name in the sidebar (prefixed with `⊞`) |
| `services` | Array of service names to include. An empty array enables all services. |

Global views can also be created at runtime with the `g` key.

## Usage

```
vista                  # start the TUI (uses profile marked "default": true, or first profile)
vista <profile>        # start the TUI with a named profile
vista init             # generate vista.json from a compose/Procfile
```

The active profile name is shown in the title bar when one is set.

## Key Bindings

| Key | Action |
|-----|--------|
| `h` / `l` | Switch to previous / next service or global view |
| `j` / `k` | Scroll logs up / down |
| `s` | Start selected service |
| `x` | Stop selected service |
| `r` | Restart selected service |
| `g` | Create a new global view (all services enabled) |
| `d` | Delete selected global view |
| `1`–`9` | Toggle service on/off in the selected global view |
| `b` | Hide / show the sidebar |
| `t` | Toggle timestamps |
| `f` | Toggle follow mode (auto-scroll) |
| `/` | Enter search mode |
| `n` / `N` | Jump to next / previous match |
| `F` | Toggle filter mode (hide non-matching lines) |
| `esc` | Clear search and return to normal mode |
| `q` / `ctrl+c` | Stop all services and quit |

When a global view is selected, the sidebar splits to show a toggle panel at the bottom indicating which services are enabled or disabled. Use `1`–`9` to toggle individual services in the view.

### Search

Press `/` to open the search bar at the bottom of the screen. Queries are treated as **case-insensitive regular expressions** — plain text works too. While typing:

- Matching lines get a `·` prefix; the current match gets a `>` prefix
- Matched text is highlighted inline (yellow for the current match, orange for others)
- The status bar shows `n/N matches · n/N: navigate · esc: clear`

Press `enter` to commit the query and use `n`/`N` to jump between matches. Press `esc` to clear the search entirely. If the regex is invalid, Vista falls back to literal substring matching and shows the error in the status bar.

Press `F` to toggle **filter mode** — non-matching lines are hidden and only matches are shown, similar to `grep`. The status bar shows `F: filter on/off` whenever a search is active. `n`/`N` navigation continues to work in filter mode.

## Project Structure

```
main.go       Entry point, config loading (services + global views), subcommand dispatch
init.go       `vista init` — generates vista.json from a Docker Compose file
model.go      Bubble Tea model (Init, Update, View)
service.go    Service struct, Start/Stop with process groups
messages.go   Custom message types, log channel listener, globalView type
ui.go         Lipgloss styles and layout helpers
```
