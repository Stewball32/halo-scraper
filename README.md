# xemu-cartographer

A per-tick memory scraper for Halo CE running inside [xemu](https://xemu.app/) (original Xbox emulator) on Linux. Reads live game state directly from xemu's process memory via `/proc/<pid>/mem`, broadcasts it over WebSocket at 30 Hz, and optionally persists match events to [PocketBase](https://pocketbase.io/).

**Use cases:** OBS overlays, stat tracking, live dashboards, replay tools, spectator tools.

---

## Requirements

- **Go 1.22+**
- **Linux** — memory access via `/proc/<pid>/mem` requires root
- **xemu** with a QMP Unix socket exposed (used once at startup for address translation)
- _(Optional)_ **PocketBase** for persistent stat storage

---

## Quick Start

```bash
# Clone
git clone https://github.com/Stewball32/xemu-cartographer
cd xemu-cartographer

# Configure
cp xemu-cartographer.toml.example xemu-cartographer.toml
$EDITOR xemu-cartographer.toml   # set qmp_sock path(s) for your xemu instance(s)

# Run (root required for /proc/<pid>/mem)
sudo go run ./cmd/cartographer

# Or build first
go build -o cartographer ./cmd/cartographer
sudo ./cartographer
```

Open `web/debug/index.html` directly in your browser to see a live feed of all WebSocket messages.

For detailed xemu setup (QMP socket, Podman containers, systemd service), see [docs/setup.md](docs/setup.md).

---

## Configuration

Copy `xemu-cartographer.toml.example` to `xemu-cartographer.toml`. The scraper looks for the config file next to its binary, then in the working directory.

| Section | Key | Default | Description |
|---|---|---|---|
| `[server]` | `addr` | `:9000` | WebSocket listen address |
| `[server]` | `tick_hz` | `30` | Max tick broadcast rate (Hz) |
| `[pocketbase]` | `enabled` | `true` | Enable PocketBase writes |
| `[pocketbase]` | `url` | `http://localhost:8090` | PocketBase base URL |
| `[performance]` | `idle_ms` | `500` | Poll interval when not in-game (ms) |
| `[performance]` | `game_ms` | `10` | Poll interval in-game (ms) |
| `[performance]` | `events` | `true` | Enable event detection (kills, deaths, etc.) |

Each `[[hosts]]` block defines one xemu instance:

```toml
[[hosts]]
name     = "xemu-host-01"
qmp_sock = "/path/to/qmp/xemu-host-01.sock"

# Optional per-host overrides:
# idle_ms = 1000
# game_ms = 33
# events  = false
```

Failed hosts are skipped at startup; remaining instances continue normally.

---

## Architecture

```
xemu-cartographer/
├── cmd/cartographer/   # Entry point: config loading, instance orchestration, poll loop
├── internal/xemu/      # PID discovery, /proc/mem lifecycle, QMP translation, pread wrappers
├── internal/halo/      # All game knowledge: offsets, types, state readers, event detection
├── internal/ws/        # WebSocket hub — multi-client broadcast, snapshot caching
├── internal/pb/        # PocketBase REST client — background drain goroutine, non-blocking
└── web/
    ├── debug/          # Live debug UI (open index.html directly in browser)
    └── tools/          # Utility pages
```

| Package | Responsibility |
|---|---|
| `internal/xemu/` | PID discovery, `/proc/mem` lifecycle, QMP translation, typed pread wrappers |
| `internal/halo/` | All game knowledge: offsets, type definitions, state readers, event detection |
| `internal/ws/` | WebSocket hub — multi-client broadcast, snapshot caching for new connections |
| `internal/pb/` | PocketBase REST client — background drain goroutine, non-blocking |
| `cmd/cartographer/` | Config loading, instance orchestration, poll loop |

### Memory access model

xemu is a regular Linux process. Xbox RAM is a contiguous anonymous allocation in its virtual address space, accessible via `/proc/<pid>/mem`.

**Startup (once per instance):**
1. Find xemu PID via `podman inspect` (or by scanning `/proc`)
2. Open QMP Unix socket, issue `gva2gpa` + `gpa2hva` to translate guest addresses to host addresses
3. Cache the base host address; close QMP
4. Open `/proc/<pid>/mem` for `pread()` calls

**Every read:**
- High GVAs (`≥ 0x80000000`): `host_addr = base + (gva - 0x80000000)`
- Low GVAs (`< 0x80000000`): translated individually at startup and cached

All game reads use `syscall.Pread()` — no QMP after startup, no xemu pausing, microseconds of overhead per tick.

### Poll loop

Per instance, per tick:
1. `ReadGameState()` — lightweight check of menu/in-game flags
2. On state change: `ReadSnapshot()` + broadcast snapshot + emit `game_start`/`game_end`
3. In-game: `ReadTick()` + broadcast tick + `DetectEvents()` + `UpdateTickState()`
4. Events broadcast to WebSocket hub and enqueued to PocketBase (fire-and-forget, 512-deep channel)

---

## WebSocket API

Connect to `ws://localhost:9000/ws` (or your configured `addr`).

All messages are JSON with the envelope:

```json
{
  "type":     "snapshot|tick|event",
  "instance": "xemu-host-01",
  "tick":     14523,
  "payload":  { ... }
}
```

See [docs/websocket.md](docs/websocket.md) for full payload schemas and all event types.

---

## Debug UI

Open `web/debug/index.html` directly in your browser (no server needed). It connects to `ws://localhost:9000/ws` and shows:
- Live player state table (position, health, shields, weapons, flags)
- Color-coded event log (newest-first, max 200 entries)

---

## Docs

- [docs/setup.md](docs/setup.md) — Full setup: xemu QMP socket, Podman, systemd service
- [docs/websocket.md](docs/websocket.md) — WebSocket API reference (all message types and payloads)
- [docs/offsets.md](docs/offsets.md) — Halo CE memory offset reference
- [docs/pocketbase.md](docs/pocketbase.md) — PocketBase schema and collection setup

---

## Building

```bash
go build ./cmd/cartographer        # build cartographer binary
go test ./...                      # run all tests
go test ./internal/halo/...        # run halo package tests only
```
