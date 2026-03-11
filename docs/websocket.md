# WebSocket API Reference

The scraper exposes a WebSocket endpoint at `ws://localhost:9000/ws` (configurable via `[server].addr`).

---

## Connection

Connect with any standard WebSocket client. On connection, the server immediately sends a `snapshot` message for each active instance so you have full state before any ticks arrive.

```js
const ws = new WebSocket("ws://localhost:9000/ws");
ws.onmessage = (e) => {
  const msg = JSON.parse(e.data);
  console.log(msg.type, msg.instance, msg.payload);
};
```

---

## Envelope

Every message is a JSON object with this top-level structure:

```json
{
  "type":     "snapshot",
  "instance": "xemu-host-01",
  "tick":     14523,
  "payload":  { ... }
}
```

| Field | Type | Description |
|---|---|---|
| `type` | string | `"snapshot"`, `"tick"`, or `"event"` |
| `instance` | string | Name of the xemu instance (from config) |
| `tick` | number | Current game tick (30 ticks/sec; 0 when not in-game) |
| `payload` | object | Message-type-specific data (see below) |

---

## Message Types

### `snapshot`

Sent on connect and on every game-state transition (menu → pregame → in_game → postgame → ...). Contains the full static/score state of the match.

```json
{
  "game_state":       "in_game",
  "map":              "bloodgulch",
  "gametype":         "slayer",
  "is_team_game":     false,
  "score_limit":      25,
  "time_limit_ticks": 54000,
  "team_scores": [
    { "team": 0, "score": 12 },
    { "team": 1, "score": 9 }
  ],
  "players": [
    {
      "index":       0,
      "name":        "PlayerOne",
      "team":        0,
      "kills":       12,
      "deaths":      4,
      "assists":     2,
      "ctf_score":   0,
      "team_kills":  0,
      "suicides":    0,
      "kill_streak": 3,
      "multikill":   0,
      "shots_fired": 340,
      "shots_hit":   180
    }
  ],
  "power_item_spawns": [
    {
      "spawn_id":             0,
      "tag":                  "powerups\\active camouflage",
      "spawn_interval_ticks": 900,
      "x": 34.2, "y": -12.5, "z": 0.8
    }
  ]
}
```

**`game_state` values:** `"menu"` | `"pregame"` | `"in_game"` | `"postgame"`

**`gametype` values:** `"ctf"` | `"slayer"` | `"oddball"` | `"king"` | `"race"` | `"terminator"` | `"none"`

---

### `tick`

Sent every game tick (up to `tick_hz` Hz, default 30). Contains dynamic per-player state: positions, health, weapons, actions.

```json
{
  "players": [
    {
      "index":               0,
      "alive":               true,
      "respawn_in_ticks":    null,
      "x": 34.2, "y": -12.5, "z": 0.8,
      "vx": 0.1, "vy": 0.0, "vz": 0.0,
      "aim_x": 0.9, "aim_y": 0.1, "aim_z": 0.0,
      "zoom_level":          -1,
      "crouchscale":         0.0,
      "health":              0.85,
      "shields":             1.0,
      "has_camo":            false,
      "has_overshield":      false,
      "frags":               2,
      "plasmas":             1,
      "selected_weapon_slot": 0,
      "is_crouching":        false,
      "is_jumping":          false,
      "is_firing":           false,
      "is_shooting":         false,
      "is_flashlight_on":    false,
      "is_throwing_grenade": false,
      "is_meleeing":         false,
      "is_pressing_action":  false,
      "is_holding_action":   false,
      "weapons": [
        {
          "slot":      0,
          "object_id": 1234,
          "tag":       "weapons\\assault rifle\\assault rifle",
          "ammo_pack": 180,
          "ammo_mag":  32,
          "charge":    null,
          "is_energy": false
        }
      ]
    }
  ],
  "power_items": [
    {
      "spawn_id":        0,
      "status":          "world",
      "held_by":         null,
      "world_pos":       { "x": 34.2, "y": -12.5, "z": 0.8 },
      "respawn_in_ticks": null
    }
  ]
}
```

**`power_items[].status` values:** `"held"` | `"world"` | `"respawning"`

**`respawn_in_ticks`** is a number when the player is dead (ticks until respawn), `null` when alive.

**`zoom_level`** is `-1` when not zooming, `0`/`1` for zoom levels.

---

### `event`

Sent when a discrete game event is detected. All event payloads include an `event_type` field identifying the event.

Player indices correspond to `snapshot.players[].index`.

#### `kill`
```json
{ "event_type": "kill", "killer": 0, "victim": 2 }
```

#### `death`
```json
{ "event_type": "death", "player": 2, "respawn_in_ticks": 90 }
```

#### `spawn`
```json
{ "event_type": "spawn", "player": 2, "x": 12.0, "y": 3.5, "z": 0.0 }
```

#### `team_kill`
```json
{ "event_type": "team_kill", "killer": 0, "victim": 1 }
```

#### `score`
Fires alongside every `kill`. Contains the killer's updated stats.
```json
{ "event_type": "score", "player": 0, "kills": 13, "deaths": 4, "assists": 2, "kill_streak": 4, "multikill": 0 }
```

#### `multikill`
Fires when `multikill` counter increases above 1.
```json
{ "event_type": "multikill", "player": 0, "count": 2 }
```

#### `kill_streak`
Fires when `kill_streak` counter increases.
```json
{ "event_type": "kill_streak", "player": 0, "count": 5 }
```

#### `damage`
```json
{ "event_type": "damage", "receiver": 2, "amount": 0.35, "dealer": 0 }
```
`dealer` may be absent if the damage source couldn't be attributed.

#### `melee`
```json
{ "event_type": "melee", "player": 0, "victim": 2 }
```
`victim` may be absent if no target was found in the damage table.

#### `grenade_thrown`
```json
{ "event_type": "grenade_thrown", "player": 0, "kind": "frag", "frags_remaining": 1 }
{ "event_type": "grenade_thrown", "player": 0, "kind": "plasma", "plasmas_remaining": 0 }
```

#### `powerup_picked_up`
```json
{ "event_type": "powerup_picked_up", "player": 0, "kind": "active_camouflage" }
{ "event_type": "powerup_picked_up", "player": 0, "kind": "overshield" }
```

#### `powerup_expired`
```json
{ "event_type": "powerup_expired", "player": 0, "kind": "active_camouflage" }
```

#### `vehicle_entered`
```json
{ "event_type": "vehicle_entered", "player": 0, "vehicle_handle": 42 }
```

#### `vehicle_exited`
```json
{ "event_type": "vehicle_exited", "player": 0 }
```

#### `item_picked_up`
```json
{ "event_type": "item_picked_up", "spawn_id": 0, "player": 1, "tag": "powerups\\active camouflage" }
```

#### `item_dropped`
```json
{ "event_type": "item_dropped", "spawn_id": 0, "tag": "powerups\\active camouflage" }
```

#### `item_spawned`
```json
{ "event_type": "item_spawned", "spawn_id": 0, "tag": "powerups\\active camouflage" }
```

#### `item_depleted`
Fires when a weapon runs out of ammo or energy.
```json
{ "event_type": "item_depleted", "player": 0, "tag": "weapons\\plasma pistol\\plasma pistol", "kind": "energy" }
{ "event_type": "item_depleted", "player": 0, "tag": "weapons\\sniper rifle\\sniper rifle", "kind": "ammo" }
```

#### `player_quit`
```json
{ "event_type": "player_quit", "player": 3 }
```

#### `game_start`
```json
{ "event_type": "game_start" }
```

#### `game_end`
```json
{ "event_type": "game_end" }
```

---

## Example: JavaScript overlay

```js
const ws = new WebSocket("ws://localhost:9000/ws");
const state = {};

ws.onmessage = ({ data }) => {
  const { type, instance, tick, payload } = JSON.parse(data);

  if (type === "snapshot") {
    state[instance] = { snapshot: payload };
    renderScoreboard(payload);
  } else if (type === "tick") {
    if (state[instance]) state[instance].tick = payload;
    renderPositions(payload.players);
  } else if (type === "event") {
    handleEvent(payload.event_type, payload, instance);
  }
};

function handleEvent(type, payload, instance) {
  if (type === "kill") {
    showKillFeed(payload.killer, payload.victim, instance);
  } else if (type === "game_start") {
    resetUI();
  }
}
```
