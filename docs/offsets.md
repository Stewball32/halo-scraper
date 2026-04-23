# Halo CE Memory Offsets

All addresses are **guest virtual addresses (GVAs)** for Halo CE (PC/Xbox). The scraper translates these to host addresses at startup — see [README.md](../README.md#memory-access-model) for the translation model.

All values are little-endian. Field types use the notation `uN`/`sN`/`fN` for unsigned/signed/float of N bits.

---

## Global Pointers (Low GVAs — need QMP translation at startup)

These addresses are below `0x80000000` and require individual `gva2gpa` → `gpa2hva` translations at startup. Read a `u32` at the translated address to get the actual data pointer.

```go
AddrPlayerDatumArrayPtr   uint32 = 0x2FAD28   // → player datum array header
AddrPlayersGlobalsPtr     uint32 = 0x2FAD20   // → players globals
AddrTeamsPtr              uint32 = 0x2FAD24   // → teams array
AddrGameGlobalsPtr        uint32 = 0x27629C   // → game globals
AddrGlobalGameGlobalsPtr  uint32 = 0x39BE4C   // → global game globals
AddrGameServerPtr         uint32 = 0x2E3628   // → game server data
AddrGameClientPtr         uint32 = 0x2E362C   // → game client data
AddrObjectHeaderDatumPtr  uint32 = 0x2FC6AC   // → object header datum array
AddrGameTimeGlobalsPtr    uint32 = 0x2F8CA0   // → game time globals
AddrGlobalTagInstancesPtr uint32 = 0x39CE24   // → global tag instances
AddrGlobalScenarioPtr     uint32 = 0x39BE5C   // → global scenario
AddrGameEngineGlobalsPtr  uint32 = 0x2F9110   // → game engine globals
AddrGameVariantGlobalPtr  uint32 = 0x2FAB60   // → game variant global
```

## Direct Addresses (High GVAs — use base offset math)

```go
AddrGameConnection       uint32 = 0x2E3684   // u16: 0=menu, 1=browsing, 2=hosting, 3=film
AddrIsTeamGame           uint32 = 0x2F90C4   // u8
AddrMainMenuActive       uint32 = 0x2E4068   // u8
AddrGameCanScore         uint32 = 0x2FABF0   // u32: 0 = can score
AddrMultiplayerMapName   uint32 = 0x2E37CD   // string
AddrGlobalStageName      uint32 = 0x2FAC20   // string (host only)
AddrGameTimeAddress      uint32 = 0x2F8CAC   // = GameTimeGlobalsPtr + 12
```

---

## Player Datum Array Header

Read from `*AddrPlayerDatumArrayPtr`.

```
+0x20  u16   max_count
+0x22  u16   element_size        (typically 0xD4 = 212 bytes)
+0x2E  u16   current_count
+0x34  u32   first_element_addr
```

---

## Static Player Struct

Base: `first_element_addr + player_index * element_size`

```
+0x02  s16   local_player_index  (-1 if remote; 0–3 = splitscreen slot; exposed as is_local / local_index in snapshot)
+0x04  [24]  name                (UTF-16LE, 12 chars max)
+0x20  u32   team                (0=red, 1=blue; 0–15 for FFA)
+0x24  u32   action_target_ref
+0x28  u16   action
+0x2A  u16   action_seat
+0x2C  u32   respawn_timer
+0x30  u32   respawn_penalty
+0x34  u32   object_ref          (0xFFFFFFFF when dead)
+0x36  u16   object_index
+0x38  u32   previous_object_ref
+0x44  u32   time_of_last_shot
+0x68  u32   camo_timer
+0x6C  f32   player_speed
+0x84  u32   time_of_last_death
+0x88  u32   target_player_index
+0x92  u16   kill_streak
+0x94  u16   multikill
+0x96  s16   time_of_last_kill   (ticks; -1 on death)
+0x98  s16   kills
+0xA0  s16   assists
+0xA8  s16   team_kills
+0xAA  s16   deaths
+0xAC  s16   suicides
+0xAE  s32   shots_fired
+0xB2  s16   shots_hit
+0xC4  s16   ctf_score
+0xD1  u8    player_quit
```

---

## Dynamic Player Object (Biped)

Located via the object header table lookup (see below). At `object_data_addr`:

```
+0x04  u32   flags
+0x0C  f32   x
+0x10  f32   y
+0x14  f32   z
+0x18  f32   vel_x
+0x1C  f32   vel_y
+0x20  f32   vel_z
+0x88  f32   max_health
+0x8C  f32   max_shields
+0x90  f32   health              (0.0–1.0)
+0x94  f32   shields             (0.0–1.0; >1.0 = overshield)
+0xB6  u16   shields_status      (0x1000=recharging, 0x8=depleted, 0x10=overshield)
+0x1B4 u8    camo                (0x41=no camo, 0x51=camo active)
+0x1B6 u8    flashlight
+0x1EC f32   aiming_vector_x
+0x1F0 f32   aiming_vector_y
+0x1F4 f32   aiming_vector_z
+0x2A2 s16   selected_weapon_index
+0x2CE u8    primary_grenades    (frags)
+0x2CF u8    secondary_grenades  (plasmas)
+0x2D0 s8    zoom_level          (-1=not zooming)
+0x32C f32   camo_amount         (0.0=none, 1.0=full)
+0x3E0 [64]  damage_table        (4 entries × 16 bytes each)
+0x424 u8    airborne            (&1=airborne, &2=slipping)
+0x464 f32   crouch_scale        (0.0=standing, 1.0=crouching)
```

### Damage table entry (16 bytes each, 4 slots at +0x3E0)

```
+0x00  u32   damage_time         (0xFFFFFFFF = empty slot)
+0x04  f32   amount
+0x08  u32   dealer_obj_handle
+0x0C  u32   dealer_plr_handle   (&0xFFFF = player index)
```

### Derived camera position

```
camera_x = dynamic.x
camera_y = dynamic.y
camera_z = dynamic.z + lerp(biped_camera_height_standing, biped_camera_height_crouching, crouch_scale)
```

---

## Observer Camera

For spectator/overlay projection. Base: `0x271550 + local_player_index * 668`

```
+0x00  f32   x
+0x04  f32   y
+0x08  f32   z
+0x14  f32   vel_x
+0x18  f32   vel_y
+0x1C  f32   vel_z
+0x20  f32   aim_x
+0x24  f32   aim_y
+0x28  f32   aim_z
+0x38  f32   fov                 (vertical, radians)
```

---

## Game Time Globals

Read from `*AddrGameTimeGlobalsPtr`.

```
+0x00  u8    initialized
+0x01  u8    active
+0x02  u8    paused
+0x0C  u32   game_time           (ticks, 30 ticks/sec; subtract 1 for current tick)
+0x10  u32   elapsed
+0x18  f32   speed               (1.0 = normal)
```

### Game state inference

```
in_game    = initialized=1 AND active=1 AND paused=0 AND main_menu=0
pregame    = initialized=1 AND active=0 AND paused=1
postgame   = game_engine_running=true AND game_can_score=false
```

---

## Score Addresses by Gametype

```go
// Gametype IDs
// 1=CTF, 2=Slayer, 3=Oddball, 4=King, 5=Race

var TeamScoreBase = map[uint32]uint32{
    1: 0x2762B4,  // CTF
    2: 0x276710,  // Slayer
    3: 0x27653C,  // Oddball
    4: 0x2762D8,  // King
    5: 0x2766C8,  // Race
}

// Team score:   s32 at TeamScoreBase[gametype] + 4 * team_id
// Player score: s32 at (TeamScoreBase[gametype] + 64) + 4 * player_id
// CTF player score: s16 from static player struct +0xC4
```

---

## Object Header Table

All live objects (players, weapons, vehicles, projectiles, etc.).

Header at `*AddrObjectHeaderDatumPtr`:

```
+0x20  u16   max_elements
+0x22  u16   element_size        (typically 12 bytes)
+0x2E  u16   allocated_count
+0x34  u32   first_element_addr
```

Per-slot entry (12 bytes), at `first_element_addr + object_id * 12`:

```
+0x00  u16   salt
+0x02  u8    flags
+0x08  u32   object_data_addr    (0x0 = empty slot)
```

Object base fields at `object_data_addr`:

```
+0x00  s16   tag_index
+0x04  u32   flags
+0x0C  f32   x
+0x10  f32   y
+0x14  f32   z
+0x18  f32   vel_x
+0x1C  f32   vel_y
+0x20  f32   vel_z
+0x64  u8    object_type
```

Object type values:
```
0x0 = object      0x5 = scenery     0xA = biped
0x1 = device      0x6 = machine     0xB = vehicle
0x2 = item        0x7 = control
0x3 = weapon      0x8 = liffblock
0x4 = projectile  0x9 = sound_scenery
```

## Xbox console name

The local xbox console name has no static GVA — it's heap-allocated by Halo's
XString allocator. We locate it by pattern-scanning GVA 0x81000000–0x83000000
for a 28-byte distinctive header that immediately precedes every heap copy:

```
C4 24 0A D0 80 B1 2F 00 08 00 00 00 6F E1 17 00
78 14 20 00 6C 24 0A D0 DE 24 00 00
```

The name follows 4 bytes after the header (a flag u32 whose value varies) as
UTF-16LE terminated by `00 00`. The header bytes are kernel pointers resolved
at XBE load time, so they should be stable across xemu boots.

See [internal/scraper/haloce/xboxname.go](../internal/scraper/haloce/xboxname.go).
Surfaced on `/api/status` as `hosts.<name>.xbox_name`.
