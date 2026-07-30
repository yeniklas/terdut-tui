# terdut-tui

A terminal user interface for [terdut-server](https://github.com/terdut-server). Communicates with the server over its REST API.

Written in Go using [Bubbletea](https://github.com/charmbracelet/bubbletea).

## Features

- **Incident queue** — open incidents with severity, status, assignee and age, auto-refreshing
- **Incident actions** — acknowledge, assign, snooze, note, resolve and archive
- **Timeline** — the full history of an incident, system events and notes together
- **Alert feed** — the raw read-only alerts underneath, each linked to its incident
- **On-call schedule** — visual calendar of who is on duty, assign and remove entries
- **Statistics** — MTTA and MTTR, plus alert frequency by name, hour and day
- **User management** — add and remove users, manage API keys

> Requires terdut-server **v0.4.0 or later**. Earlier servers have no incidents API;
> use terdut-tui v0.3.x with those.

## Alerts and incidents

The server keeps two objects and this client follows that split:

- An **alert** is Alertmanager's record — firing or resolved, and read-only here.
- An **incident** is the work item. It is what you acknowledge, assign, snooze,
  discuss and resolve, and it is where all the actions live.

Incidents are correlated by the `groupKey` Alertmanager already computed from your
`group_by` configuration, so several alerts commonly share one incident.

Two behaviours worth knowing before you press a key:

- **Resolving is final.** The server treats a manual resolve as terminal: a later
  occurrence opens a *new* incident rather than reopening this one, and if the alert
  underneath never stops firing the incident stays closed. The TUI asks for
  confirmation before doing it.
- **Snooze is the "not now" button.** It hides an incident from the default queue
  without closing it, and expires on its own.

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

Global:

| Key | Action |
|-----|--------|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `tab` / `shift+tab` | Next / previous section |
| `enter` | Open detail |
| `esc` | Go back |
| `r` | Refresh |
| `f` | Cycle filter |
| `S` | Statistics |
| `q` | Quit |

Incidents section:

| Key | Action |
|-----|--------|
| `f` | Cycle: open → triggered → acknowledged → resolved → snoozed |
| `x` | Archive (resolved incidents only) |

Incident detail:

| Key | Action |
|-----|--------|
| `a` / `A` | Acknowledge / clear acknowledgement |
| `R` | Resolve — asks to confirm, and is final |
| `s` | Assign to a user |
| `z` / `Z` | Snooze for a duration / un-snooze |
| `c` | Add a note |
| `[` / `]` | Select a note |
| `d` | Delete the selected note (your own only) |
| `x` | Archive / un-archive |

Alerts section (read-only):

| Key | Action |
|-----|--------|
| `f` | Cycle: firing → resolved → all → archived |
| `i` | In detail: jump to the alert's incident |

Schedule section:

| Key | Action |
|-----|--------|
| `+` / `W` | Assign a day / a whole week |
| `d` | Remove the assignment |
| `←` / `→` | Shift the week window |

Users section:

| Key | Action |
|-----|--------|
| `n` | Create a user |
| `d` | Delete a user |
| `k` | API keys for the selected user |
