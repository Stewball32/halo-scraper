# PocketBase Setup

PocketBase is optional. Set `enabled = false` in `xemu-cartographer.toml` if you don't need persistent stat storage.

---

## Running PocketBase

```bash
# Download from https://pocketbase.io/docs/
./pocketbase serve --http="localhost:8090"
```

Open `http://localhost:8090/_/` to access the admin UI and create the collections below.

---

## Collections

### `sessions`

One record per match.

| Field | Type | Notes |
|---|---|---|
| `start_time` | DateTime | Match start timestamp |
| `end_time` | DateTime | Match end timestamp (set on `game_end`) |
| `map` | Text | Map name (e.g. `bloodgulch`) |
| `gametype` | Text | Gametype string (e.g. `slayer`) |
| `instance_id` | Text | xemu instance name (e.g. `xemu-host-01`) |

### `snapshots`

State snapshots linked to a session. Written on game-state transitions, not every tick.

| Field | Type | Notes |
|---|---|---|
| `session` | Relation → `sessions` | |
| `tick` | Number | Game tick at snapshot time |
| `data` | JSON | Full `SnapshotPayload` as JSON |

### `players`

Player registry — one record per unique player.

| Field | Type | Notes |
|---|---|---|
| `name` | Text | Gamertag |
| `eeprom_id` | Text | EEPROM-derived hardware ID (unique per Xbox) |
| `total_kills` | Number | Aggregate across all sessions |
| `total_deaths` | Number | |
| `total_assists` | Number | |
| `total_matches` | Number | |

### `events`

Discrete game events. One record per event emission.

| Field | Type | Notes |
|---|---|---|
| `session` | Relation → `sessions` | |
| `tick` | Number | |
| `instance_id` | Text | xemu instance name |
| `event_type` | Text | e.g. `kill`, `death`, `spawn` — see [websocket.md](websocket.md) |
| `data` | JSON | Full event payload as JSON |

### `overlay_state`

One record **per xemu instance**, updated in-place each tick. Used by overlays that want to poll PocketBase instead of maintaining a WebSocket connection.

| Field | Type | Notes |
|---|---|---|
| `instance_id` | Text | xemu instance name (use as unique key) |
| `tick` | Number | |
| `data` | JSON | Latest `TickPayload` as JSON |

> **Important:** Create one `overlay_state` row per instance and update it in-place (don't insert new rows every tick). Sharing a single row across instances causes SQLite write contention.

---

## Notes

- PocketBase writes are fire-and-forget. The scraper enqueues events to a 512-deep channel and a background goroutine drains it. If PocketBase is slow or down, the channel fills and events are dropped rather than blocking the poll loop.
- The scraper uses the PocketBase REST API (no SDK). Auth is not currently implemented — run PocketBase on localhost and restrict external access at the network level if needed.
