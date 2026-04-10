# lazytalos

A keyboard-driven terminal UI for Talos Linux clusters.

[Installation](#installation) &middot;
[Features](#features) &middot;
[Keybindings](#keybindings) &middot;
[Configuration](#configuration) &middot;
[License](#license)

---

**lazytalos** is a fast, keyboard-first TUI for managing Talos Linux clusters from the terminal. It follows the "lazy" convention ([lazygit](https://github.com/jesseduffield/lazygit), [lazydocker](https://github.com/jesseduffield/lazydocker), [lazystack](https://github.com/larkly/lazystack)) to provide an intuitive alternative to `talosctl` and the built-in Talos dashboard.

Single binary. No runtime dependencies. Reads your standard `talosconfig`.

## Features

- **Cluster dashboard** -- dot-matrix node health overview, service status matrix, diagnostics, and event stream across all nodes
- **Node management** -- list all nodes with type, version, health, CPU, memory bars, and uptime; bulk-select with `Space` for multi-node operations; detail view with live log streaming
- **Service management** -- flat or group-by-node service listing with state, health, and event history; restart services with confirmation
- **Multi-node log viewer** -- merged streaming logs from multiple nodes and services simultaneously, with per-node color coding and follow mode
- **Container view** -- running containers across all nodes
- **Network view** -- network interfaces, addresses, and routes per node
- **Storage view** -- disk and volume information per node
- **etcd view** -- etcd cluster member listing with health and leader status
- **Cluster upgrade wizard** -- rolling upgrade with pause/abort, health checks between nodes
- **Context picker** -- read all contexts from `talosconfig`; auto-connect when only one exists, or pick interactively
- **Settings overlay** (`Ctrl+K`) -- configure refresh interval, colors, thresholds, keybindings; persisted to `~/.config/lazytalos/config.yaml`
- **Configurable thresholds** -- memory warning/critical and CPU warning percentages for bar coloring
- **Auto-update check** -- checks GitHub for new releases (configurable interval, cached to disk)
- **Destructive-op safety** -- reboot, shutdown, reset, and service restart require confirmation modals; control plane nodes trigger extra warnings
- **Auto-refresh** -- all views refresh in the background at a configurable interval

## Installation

### Pre-built binaries

Grab the latest release for your platform from the [releases page](https://github.com/larkly/lazytalos/releases/latest).

### Debian / Ubuntu

```bash
sudo dpkg -i lazytalos_*.deb
```

### Fedora / RHEL

```bash
sudo rpm -i lazytalos-*.rpm
```

### Arch Linux

```bash
sudo pacman -U lazytalos-*.pkg.tar.zst
```

### From source

```bash
cd src
make build
```

### With `go install`

```bash
go install github.com/larkly/lazytalos/cmd/lazytalos@latest
```

### Requirements

- Go 1.24+ (build only)
- A Talos Linux cluster (v1.x+)
- A valid `talosconfig`

## Configuration

lazytalos reads `talosconfig` from these locations (first match wins):

1. `--talosconfig` flag
2. `$TALOSCONFIG` environment variable
3. `~/.talos/config`

No additional configuration is needed. If only one context is defined, lazytalos connects automatically.

### Settings overlay

Press `Ctrl+K` to open the settings overlay. Changes are applied immediately and saved to `~/.config/lazytalos/config.yaml`. Categories:

- **General** -- refresh interval, plain mode, update checking
- **Thresholds** -- memory warning/critical %, CPU warning % (controls bar and dot-matrix coloring)
- **Colors** -- full Solarized Dark palette customization with hex colors and live preview
- **Keybindings** -- press-to-bind key remapping

### CLI flags

CLI flags override the config file.

| Flag | Default | Description |
|------|---------|-------------|
| `--talosconfig PATH` | `$TALOSCONFIG` or `~/.talos/config` | Path to talosconfig file |
| `--context NAME` | | Connect directly to named context, skip picker |
| `--pick-context` | `false` | Always show context picker, even with one context |
| `--refresh N` | `5` | Auto-refresh interval in seconds |
| `--plain` | `false` | Disable Unicode status icons |
| `--debug` | `false` | Write debug log to `~/.cache/lazytalos/debug.log` |
| `--no-update-check` | `false` | Disable startup update check |
| `--update-check-interval N` | `24` | Hours between GitHub API update checks (0 = every launch) |
| `--version` | | Print version and exit |

## Keybindings

### Global

| Key | Action |
|-----|--------|
| `q` / `Ctrl+C` | Quit |
| `1`--`8` / `Left` / `Right` | Switch tab |
| `C` | Switch context |
| `?` | Help overlay |
| `Ctrl+K` | Settings overlay |
| `Ctrl+,` | View talosconfig contexts |
| `Ctrl+R` | Force refresh |
| `/` | Filter |
| `s` | Sort column |
| `PgUp` / `PgDn` | Page up / down |

### Dashboard

| Key | Action |
|-----|--------|
| `F` | Toggle event follow mode |
| `Up` / `Down` | Scroll events |

### Nodes

| Key | Action |
|-----|--------|
| `j` / `k` | Navigate |
| `Enter` | View node detail (with live logs) |
| `Space` | Select / deselect node |
| `A` | Select all / deselect all |
| `y` | Copy node IP |
| `Y` | Copy endpoint |
| `Ctrl+O` | Reboot selected node(s) |
| `Ctrl+D` | Shutdown selected node(s) |
| `Ctrl+U` | Upgrade cluster |
| `Esc` | Back to list |

#### Node detail

| Key | Action |
|-----|--------|
| `Up` / `Down` | Scroll logs |
| `PgUp` / `PgDn` | Page scroll logs |
| `F` | Toggle log follow mode |
| `Ctrl+X` | Reset node |
| `Esc` | Back to list |

### Services

| Key | Action |
|-----|--------|
| `j` / `k` | Navigate |
| `Enter` | View service detail |
| `g` | Toggle group-by-node |
| `Ctrl+J` | Restart service |
| `Esc` | Back to list |

### Logs

| Key | Action |
|-----|--------|
| `Tab` / `Shift+Tab` | Switch between selectors and log pane |
| `Space` | Toggle node or service selection |
| `F` | Toggle follow mode |

### Confirmation dialog

| Key | Action |
|-----|--------|
| `Ctrl+S` | Confirm action |
| `Esc` | Cancel |

## Architecture

Built with:

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) v2 -- TUI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) v2 -- styling
- [Bubbles](https://github.com/charmbracelet/bubbles) v2 -- UI components
- [Talos machinery](https://github.com/siderolabs/talos/tree/main/pkg/machinery) -- official Talos gRPC client

```
src/internal/
  app/            # Root model, routing, actions, rendering
  config/         # App settings (config.yaml) + Talos config helpers
  talos/          # Talos gRPC client wrapper, context management
  cluster/        # Cluster-wide queries (members, health, targets)
  resources/      # COSI resource helpers (CPU, memory, diagnostics)
  update/         # Self-update check with GitHub API + disk cache
  shared/         # Keys, styles, messages, thresholds, debug logging
  ui/
    contextpicker/  # Context selection modal
    dashboard/      # Cluster health dashboard with dot matrix
    nodelist/       # Node list with detail view + live logs
    servicelist/    # Service list with grouping
    logviewer/      # Multi-node log streaming
    containers/     # Container listing
    network/        # Network interfaces and routes
    storage/        # Disk and volume info
    etcdview/       # etcd cluster members
    settings/       # Settings overlay (Ctrl+K)
    configview/     # Talosconfig context viewer
    configeditor/   # Node config YAML editor
    upgrade/        # Cluster upgrade wizard
    help/           # Help overlay
    modal/          # Confirm, error, and reset dialogs
    statusbar/      # Bottom status bar
```

## License

[Apache 2.0](LICENSE)
