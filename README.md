# terdut-tui

A terminal user interface for [terdut-server](https://github.com/terdut-server). Communicates with the server over its REST API.

Written in Go using [Bubbletea](https://github.com/charmbracelet/bubbletea).

## Features

- **Alert dashboard** — live view of firing and resolved alerts with auto-refresh
- **Alert actions** — acknowledge, comment, and view per-alert statistics
- **On-call schedule** — visual calendar of who is on duty, assign and remove entries
- **User management** — add and remove users, manage API keys

## Installation

Download the latest release binary for your platform from the [releases page](https://github.com/yeniklas/terdut-tui/releases), or build from source:

```bash
go install github.com/yeniklas/terdut-tui@latest
```

## Configuration

Create `~/.config/terdut-tui/config.yaml`:

```yaml
server_url: https://terdut.example.com
api_key: <your-api-key>
refresh_interval: 30  # seconds, optional
```

The API key is generated in terdut-server. See the server documentation for how to bootstrap a user and issue an API key.

## Usage

```
terdut-tui                 start the TUI
terdut-tui --version       print version
terdut-tui --self-update   update to the latest release
```

### Keybindings

| Key | Action |
|-----|--------|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `tab` | Switch section (Alerts / Schedule / Users) |
| `enter` | Select / open detail |
| `esc` | Go back |
| `r` | Refresh |
| `f` | Filter / cycle filter |
| `q` | Quit |
