# lazytalos — Product Requirements Document

## Overview

**lazytalos** is a keyboard-driven terminal UI for Talos Linux clusters, built with Go. It follows the "lazy" convention (lazygit, lazydocker, lazystack) to provide a fast, keyboard-first alternative to `talosctl` and the built-in Talos dashboard. The goal is to make day-to-day Talos cluster operations — health checking, log tailing, service management, config patching, and upgrades — fast and intuitive from the terminal.

**License**: Apache 2.0  
**Language**: Go  
**Target**: Any Talos Linux cluster reachable via `talosconfig` (v1.x+; developed against v1.12.4)  
**API client**: `github.com/siderolabs/talos/pkg/machinery` (gRPC, no `talosctl` binary dependency at runtime)

## Implementation Status

| Phase | Status | Notes |
|-------|--------|-------|
| Phase 1: MVP | Not started | talosconfig connect, context picker, dashboard, node list, services, multi-node log viewer |
| Phase 2: Resources | Not started | Containers tab, Network tab, Storage tab |
| Phase 3: etcd + Config | Not started | etcd tab, per-node inline config editor with validation |
| Phase 4: Upgrades + Dangerous Ops | Not started | Sequential upgrade orchestration, reset/wipe with heavy confirmation |
| Phase 5: Quality of Life | Not started | Filtering, sorting, clipboard, self-update check, config view |

## Problem Statement

Talos Linux operators lack a fast, keyboard-driven terminal interface for cluster-level operations:

- **`talosctl`** requires specifying `--nodes` on every command. Running the same command across all nodes is tedious and error-prone.
- **The built-in `talosctl dashboard`** is read-only and single-node at a time. It does not aggregate across nodes or support any write operations.
- **No at-a-glance cluster health view exists** — you must chain multiple `talosctl` commands (`service`, `get members`, `memory`, `health`) to understand cluster state.
- **Log tailing is fragmented** — you must run separate `talosctl logs --nodes <ip> --tail` calls per node per service. There is no unified multi-node log stream.
- **Config management is CLI-only** — `talosctl edit` opens `$EDITOR` in a subprocess, but there is no TUI-native experience with validation feedback and apply confirmation.

lazytalos fills this gap: a single binary that reads your existing `talosconfig`, connects to any Talos cluster, and presents a navigable, auto-refreshing TUI for all common operations.

## Core Principles

1. **Keyboard-first**: Every action is reachable via keyboard. Mouse is not a goal.
2. **Multi-node by default**: All views aggregate across nodes. Node-specific drill-down is secondary.
3. **Safety by friction**: Destructive operations (reboot, reset, upgrade, wipe) require explicit multi-step confirmation. Ctrl-prefixed bindings for all write actions.
4. **Read talosconfig, nothing else**: No separate config file to set up. Works out of the box if `talosconfig` is in `~/.talos/config` or pointed to via `TALOSCONFIG` env.
5. **No runtime dependencies**: Single statically-linked binary. Does not shell out to `talosctl`.
6. **Open source, community-first**: Apache 2.0. Designed for the broad Talos operator community from day one.

## Architecture

### Technology Stack

| Component | Choice | Rationale |
|-----------|--------|-----------|
| TUI framework | Bubble Tea v2 | Same as lazystack; proven Elm-like architecture |
| Styling | Lip Gloss v2 | Same as lazystack; matches project conventions |
| Talos API | `siderolabs/talos/pkg/machinery` | Official Go gRPC client; no `talosctl` binary dependency |
| Config parsing | `talosconfig` via machinery client | Native talosconfig format; multi-context support |
| Build | Go 1.22+ | Matches Talos toolchain |

### Talos API Client Model

Unlike OpenStack (REST/gophercloud), the Talos API is gRPC-based. Key differences that shape the architecture:

- **Per-node connections**: Each node endpoint is a separate gRPC connection. The machinery client handles fan-out but responses carry the originating node address.
- **Streaming responses**: Logs and events are gRPC server-side streams. These require goroutine-per-stream management rather than polling.
- **Resource API**: `talosctl get` maps to `ResourcesClient.List()` and `ResourcesClient.Watch()`. Resources are namespace + type + ID tuples.
- **Context switching**: `talosconfig` contexts map to different endpoint sets. The machinery client supports `WithConfigContext()` to switch contexts.

### Model/Update/View Pattern (Bubble Tea v2)

Identical architecture to lazystack:
- Value receivers on all `Update()` methods
- Single tick source at root model
- Background tick routing to all initialized tabs
- Modal overlays that do not swallow background ticks
- `pendingAction` pattern for optimistic UI on slow gRPC calls

### Node Targeting

Operations can target a subset of nodes via space-bar selection in the Nodes tab. The selection is a `map[string]bool` keyed by node IP. When a write action is triggered:

- If no nodes are selected: action targets the cursor node only
- If nodes are selected: action targets all selected nodes with a bulk confirmation dialog showing the node list

Control plane vs. worker targeting awareness: the confirmation dialog warns when a reboot/reset targets control plane nodes.

### Internal Package Structure (planned)

```
src/
  cmd/lazytalos/       # main.go, CLI flags, program entrypoint
  internal/
    app/               # root model, routing, tab management, action handlers
    talos/             # machinery client wrapper, context management
    cluster/           # member listing, health aggregation
    node/              # per-node resource fetching (services, processes, memory, etc.)
    resources/         # typed wrappers around the Talos resource API
    etcd/              # etcd member management, snapshot
    logs/              # multi-node streaming log aggregation
    upgrade/           # sequential upgrade orchestration
    config/            # machine config fetch, validate, apply
    shared/            # keybindings, styles, message types
    ui/                # all view and modal packages
```

## Feature Specification by Phase

---

### Phase 1: MVP

**Goal**: Connect to a cluster, see health at a glance, tail logs. Useful on day one.

#### Context Picker (startup / `C` key)

- On startup: read `talosconfig` (from `TALOSCONFIG` env, `~/.talos/config`, or `--talosconfig` flag)
- If multiple contexts exist: show a picker modal identical to lazystack's cloud picker
- If one context: connect immediately
- `C` key from any view: open context picker to switch clusters without restart
- Status bar shows current context name

#### Tab: Dashboard (`1`)

The primary view and the answer to "is my cluster healthy right now?"

**Node health matrix** (top section):
```
CLUSTER: tnn3-demo   6 nodes   3 control plane / 3 worker   Talos v1.12.4
─────────────────────────────────────────────────────────────────────────
NODE              TYPE          STATE    HEALTH   CPU    MEM    UPTIME
tnn3-demo-cp-1    controlplane  Running  OK       4.2%   38%    17d
tnn3-demo-cp-2    controlplane  Running  OK       3.8%   35%    17d
tnn3-demo-cp-3    controlplane  Running  OK       4.1%   37%    8d
tnn3-demo-worker-1 worker       Running  OK       12.1%  62%    17d
tnn3-demo-worker-2 worker       Running  OK       9.4%   58%    17d
tnn3-demo-worker-3 worker       Running  OK       11.2%  61%    17d
```

**Service matrix** (middle section):
```
SERVICE      cp-1  cp-2  cp-3  w-1  w-2  w-3
apid          OK    OK    OK    OK   OK   OK
auditd        OK    OK    OK    OK   OK   OK
containerd    OK    OK    OK    OK   OK   OK
etcd          OK    OK    OK    -    -    -
kubelet       OK    OK    OK    OK   OK   OK
machined      OK    OK    OK    OK   OK   OK
```
Services only present on some node types show `-` for nodes that don't run them.

**Events stream** (bottom section):
- Last N events from all nodes, merged and sorted by timestamp
- Color-coded by node
- Auto-scrolling with `F` to toggle follow/freeze

Metrics (CPU, memory) are fetched from `talosctl memory` and `talosctl cgroups` equivalents via the machinery API. Auto-refreshes every 5s.

#### Tab: Nodes (`2`)

Full node list with space-selection for bulk operations.

Columns: hostname, machine type, Talos version, addresses (IPv4 primary), health score, last event time.

**Actions on selected node(s)**:
- `Ctrl+O` — Reboot (confirmation: shows selected node list, requires `Ctrl+S`)
- `Ctrl+D` — Shutdown (confirmation required)
- `Ctrl+K` — Restart a service (opens service picker modal)
- `Enter` — Node detail view (version, config status, addresses, all services, recent events)

Control plane nodes show a `[CP]` badge. The confirmation dialog for reboot/shutdown of a control plane node adds a warning: "Rebooting a control plane node may cause brief etcd leadership re-election."

#### Tab: Services (`3`)

Flat list of all services across all nodes. One row per node+service combination.

Columns: node, service, state, health, last change, last event.

Filter by: node (`n`), service name (`/`), state (`s`).

**Actions**:
- `Ctrl+R` — Restart service on that node (confirmation required)
- `Enter` — Service detail: full event log for that service on that node

#### Tab: Logs (`4`)

Multi-node merged log stream — the killer feature.

**Node/service selector** (left pane, 30% width):
```
NODES          SERVICES
[x] cp-1       [x] kubelet
[x] cp-2       [x] etcd
[x] cp-3       [ ] apid
[ ] worker-1   [ ] containerd
[ ] worker-2   [x] machined
[ ] worker-3
```

**Log stream** (right pane, 70% width):
```
[cp-1   ][kubelet ] I0406 15:32:01 syncLoop: sync triggered
[cp-2   ][etcd    ] {"level":"info","msg":"applied config"}
[cp-1   ][machined] starting periodic config reload
[cp-3   ][kubelet ] I0406 15:32:02 syncLoop: sync triggered
```

- Each node gets a distinct color (Lip Gloss ANSI colors, up to 6 colors for 6 nodes)
- `F` — toggle follow mode (auto-scroll to bottom)
- `?` — filter/search overlay
- `K` — clear screen
- Streaming via gRPC `talosctl logs` equivalent; goroutines per active node+service combo
- On node/service deselect: stream goroutine is stopped, prefix line printed: `[cp-1][kubelet] --- stream closed ---`

---

### Phase 2: Resources

#### Tab: Containers (`5`)

List all containers across selected nodes. Wraps `talosctl containers` (CRI namespace).

Columns: node, namespace, name, image (short), state, PID.

**Actions**:
- `Enter` — Container detail: full image ref, resource stats (CPU, memory from `stats`), mounts
- `Ctrl+L` — View logs for container (opens log viewer scoped to this container)

#### Tab: Network (`6`)

Curated view of network resources via the Talos resource API:

- **Addresses** (`addressstatuses`): interface, address, scope, flags per node
- **Hostnames** (`hostnamestatuses`): hostname per node
- **Routes** (`routestatuses`): destination, gateway, interface, metric
- **DNS upstreams** (`dnsupstreams`): upstream resolver addresses

Sub-tabs within the Network tab (left pane navigation): Addresses | Routes | DNS

#### Tab: Storage (`7`)

Curated view of storage resources:

- **Block devices** (`blockdevices`): device name, size, type (disk/partition), bus
- **Discovered volumes** (`discoveredvolumes`): filesystem type, UUID, label, size
- **Volume statuses** (`volumestatuses`): mount status, filesystem, phase
- **Disks** (`disks`): model, serial, size

Sub-tabs: Devices | Volumes

---

### Phase 3: etcd + Config Editor

#### Tab: etcd (`8`)

Read-heavy etcd management view.

**Members sub-tab**:
- etcd member list across control plane nodes (`etcdmembers`)
- Columns: member ID, peer address, client address, is learner, is leader
- Leader highlighted

**Config sub-tab**:
- Current etcd config per control plane node (`etcdconfigs`, `etcdspecs`)

**Actions** (control plane nodes only, heavy confirmation):
- `Ctrl+M` — Remove etcd member (requires typing member ID to confirm; for disaster recovery)
- Snapshot not included in MVP (complexity, output destination unclear for TUI context)

#### Config Editor (`Ctrl+E` from Node detail)

Per-node inline config editor. Modeled after `talosctl edit` but with TUI-native validation.

**Flow**:
1. Select a node in the Nodes tab, press `Ctrl+E`
2. Machine config fetched via `MachineConfigClient.Get()` and decoded to YAML
3. Full-screen TUI editor (scrollable `textarea` via Bubbles) opens with the YAML content
4. Syntax highlighting for YAML (Lip Gloss color rules, no external dep)
5. `Ctrl+V` — Validate: send config to `talosctl validate` equivalent, show errors inline
6. `Ctrl+S` — Apply with mode picker: `apply` (reboot), `no-reboot`, `staged`
7. `Esc` — Discard and return to node detail

The editor always operates on a single node. Multi-node config differences are managed by editing nodes one by one. The status bar shows which node is being edited throughout.

**Validation errors** shown in a bottom panel before apply:
```
✗ machine.network.hostname: must be a valid RFC1123 hostname
✗ machine.install.disk: /dev/xvdb does not exist on this node
```

---

### Phase 4: Upgrades + Dangerous Ops

#### Upgrade Orchestration (`Ctrl+U` from Nodes tab)

Sequential rolling upgrade with health-checked gating. This is the most complex and highest-risk operation in the TUI.

**Upgrade wizard flow**:

1. **Target selection**: Pre-filled with all selected nodes, or all nodes if none selected. CP nodes listed first.
2. **Image selection**: Text input for the Talos installer image (e.g. `ghcr.io/siderolabs/installer:v1.13.0`). Validates format.
3. **Options**: `--preserve` (keep ephemeral data), `--stage` (stage upgrade, apply on reboot).
4. **Order preview**: Shows upgrade sequence — workers first, then CPs. User can reorder.
5. **Confirmation**: Type the cluster name to confirm (typed-confirmation pattern for high-risk ops).
6. **Execution view**:

```
UPGRADE PROGRESS  tnn3-demo → v1.13.0

[DONE]  tnn3-demo-worker-1   ████████████████████  upgraded  ↑ health OK
[DONE]  tnn3-demo-worker-2   ████████████████████  upgraded  ↑ health OK
[ACTIVE] tnn3-demo-worker-3  ████████░░░░░░░░░░░░  upgrading... rebooting
[ ]     tnn3-demo-cp-1       ░░░░░░░░░░░░░░░░░░░░  waiting
[ ]     tnn3-demo-cp-2       ░░░░░░░░░░░░░░░░░░░░  waiting
[ ]     tnn3-demo-cp-3       ░░░░░░░░░░░░░░░░░░░░  waiting

Ctrl+P  Pause after current node    Ctrl+C  Abort (leaves remaining nodes at old version)
```

- After each node's upgrade, `talosctl health` (single-node) is polled until OK before proceeding
- If health check fails after timeout: upgrade pauses and shows error; operator must manually resume or abort
- Abort leaves already-upgraded nodes at the new version (not rolled back)

#### Reset / Wipe

Available from Node detail only (not from bulk node list), to prevent accidental mass reset.

`Ctrl+X` — Reset node. Requires:
1. Confirmation modal: "Type the node hostname to confirm reset"
2. Mode selection: `graceful` (default) or `no-graceful`
3. Wipe selection: `system disk`, `user disks`, or `both`

`Ctrl+W` — Wipe block device. Not included in MVP; too dangerous without understanding target disk.

---

### Phase 5: Quality of Life

- **Filtering and sorting** on all list views (`/` for filter, `s` cycle sort column)
- **Clipboard support** (`y` to yank node IP, `Y` to yank full talosconfig context endpoint)
- **Help overlay** (`?`) — context-sensitive key binding reference, same as lazystack
- **Self-update check** — on startup, check GitHub releases for newer version (opt-out with `--no-update-check`)
- **Config view** (`Ctrl+,`) — read-only view of parsed talosconfig (contexts, endpoints, CA fingerprints)
- **Diagnostics** (`Ctrl+D` on cluster dashboard) — surface `diagnostics.runtime.talos.dev` resources, which contain Talos's own self-diagnostics
- **Plain mode** (`--plain`) — disable Unicode status icons for terminals without font support
- **Debug logging** (`--debug`) — verbose gRPC + API logging to stderr

---

## Global Key Bindings

| Key | Action |
|-----|--------|
| `1–8` | Switch to tab N |
| `←` / `→` | Previous / next tab |
| `↑` / `↓` | Navigate list |
| `PgUp` / `PgDn` | Scroll page |
| `Space` | Select/deselect node (Nodes tab) |
| `A` | Select all / deselect all |
| `/` | Filter |
| `s` | Cycle sort column |
| `Enter` | Detail view |
| `Esc` | Back / close modal |
| `C` | Context picker (switch cluster) |
| `?` | Help overlay |
| `Q` | Quit |
| `Ctrl+R` | Refresh current view now |
| `Ctrl+,` | Config view |

Write actions use `Ctrl+` prefix in all views to prevent accidental trigger:

| Key | Action | Confirmation |
|-----|--------|-------------|
| `Ctrl+O` | Reboot node(s) | Modal + `Ctrl+S` |
| `Ctrl+D` | Shutdown node(s) | Modal + `Ctrl+S` |
| `Ctrl+K` | Restart service | Modal + `Ctrl+S` |
| `Ctrl+E` | Edit node config | Full editor |
| `Ctrl+U` | Upgrade cluster | Multi-step wizard |
| `Ctrl+X` | Reset node | Typed hostname confirm |
| `Ctrl+M` | Remove etcd member | Typed member ID confirm |

---

## CLI Flags

```
lazytalos [flags]

Flags:
  --talosconfig string    Path to talosconfig (default: $TALOSCONFIG or ~/.talos/config)
  --context string        Use a specific context from talosconfig
  --refresh int           Auto-refresh interval in seconds (default: 5)
  --no-update-check       Disable startup check for newer releases
  --plain                 Disable Unicode icons (for limited terminal fonts)
  --debug                 Enable verbose debug logging to stderr
  -h, --help              Help
```

---

## Concerns and Considerations

### gRPC vs REST

The Talos machinery client is gRPC, not REST. This has architectural implications vs. lazystack:

- **Fan-out**: A single `talosctl get members` call fan-outs to all endpoints in the context. Responses come back with a `Metadata.Hostname` field. Views must aggregate by node rather than by response.
- **Streaming for logs**: Log tailing uses server-side streaming (`MachineClient.Logs()`). This means goroutine management is required — one goroutine per active node+service stream, forwarding to a Bubble Tea channel. The root model must drain this channel and route `LogLineMsg` to the Logs tab.
- **Connection state**: gRPC connections can fail silently. The machinery client has a reconnect policy, but the TUI must surface degraded node connections (e.g., gray out a node when its gRPC connection is unhealthy).
- **Context deadline management**: All gRPC calls must carry a `context.Context` with timeout. Long-running operations (upgrade health polling) need their own cancellable contexts.

### Multi-Node Aggregation Pattern

The standard Talos resource API fan-out returns duplicated rows (one per source node in `talosctl get members` shows every member from every node's perspective). Views must deduplicate using the resource `ID` field while preserving the `NODE` field for provenance. The node-keyed response model is different from OpenStack where each resource is unique.

### Config Editor Complexity

A full inline YAML editor is significantly more complex than anything in lazystack:
- Requires a text area component (Bubbles `textarea`) with scrolling
- YAML syntax validation before apply (can use Go's `gopkg.in/yaml.v3` for parse check, machinery's own validator for semantic check)
- The machine config YAML can be 200-500 lines — scrolling, line numbers, and cursor tracking are essential
- Must handle the case where the config fetch fails (node unreachable during edit)
- Must not apply if validation fails

Consider deferring to Phase 3 strictly and ensuring Phase 1-2 are stable before touching this.

### Upgrade Orchestration State Machine

The sequential upgrade is effectively a state machine with nodes as entities:

```
Pending → Upgrading → WaitingHealth → Done
                    ↘ Error (paused)
```

This state must survive Bubble Tea re-renders (value receiver pattern) and must be cancellable. Model the upgrade state as a struct in the root app model, not inside the upgrade view, so it persists through view switches. Progress must be visible even if the user switches tabs during an upgrade.

### Safety for Destructive Operations

`reset` and `wipe` are OS-level destructive operations — they make nodes permanently unrecoverable without reprovisioning. Additional safeguards beyond lazystack's confirm modal:
- Typed-name confirmation (not just `Ctrl+S`)
- Reset is available only from single-node detail view, not from bulk node list
- Wipe is deferred entirely to Phase 4 and requires explicit design review before implementation

### Version Skew

The cluster in development runs v1.12.4 while the talosctl client is v1.12.6. The machinery client should be pinned to the same minor version as the oldest cluster version intended to be supported. For v1.x, the Talos API is stable within a minor version. Consider a startup version compatibility check that warns (but does not block) on minor version mismatch.

### etcd Operations

etcd member removal is a recovery-only operation. The TUI should not make it discoverable from normal navigation — only expose it in the etcd tab with a prominent warning. The confirmation should require typing the etcd member ID (hex string), which makes accidental triggering essentially impossible.

---

## Non-Goals (v1)

- **VM / workload management**: lazytalos manages the OS layer (Talos), not Kubernetes workloads. Use `kubectl` TUIs (k9s, etc.) for pod/deployment management.
- **Metrics history / graphs**: No time-series. Current state only. Grafana/Prometheus for historical metrics.
- **Multi-cluster simultaneous view**: Context picker switches the active cluster; it does not show two clusters side-by-side.
- **OIDC / SideroLink auth**: Initially targets standard talosconfig client certificate auth only.
- **Windows support**: Linux and macOS only for v1. Windows terminal compatibility is a Phase 5+ concern.
- **AUR / Homebrew packaging**: v1 ships as GitHub binary releases only. AUR and Homebrew after stable v1.0.
