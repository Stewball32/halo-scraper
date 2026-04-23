package halo2

// Halo 2 Xbox XBE memory layout.
//
// Verification key:
//   VERIFIED   — confirmed via correlated memscan across test1/test2/test3 RAM dumps
//   UNVERIFIED — from HaloCaster source, not independently confirmed in xemu
//
// Offsets determined empirically via memscan (scanning for known UTF-16LE
// gamertags and ASCII scenario paths in xemu RAM, plus correlated u16 pattern
// scanning for kills/deaths/assists). The original HaloCaster-sourced game
// stats offsets were incorrect for xemu's memory layout; session players,
// variant info, and game_results_globals offsets have been verified.
//
// All data is accessed via constructed high GVAs (>= 0x80000000) so that the
// existing pread-based Mem reader can address them directly.

// ---------------------------------------------------------------------------
// Session players (runtime array)
// ---------------------------------------------------------------------------
//
// Array of per-player session structs, stride 0x10C.
// Each struct contains two copies of the player name (UTF-16LE),
// network identifiers (IP, XUID), color, skill, and other session metadata.
//
// WARNING: This GVA is unstable — the runtime session player array can
// relocate between game sessions. Prefer game_results_globals session
// player copies (GRGSessionPlayersOff) for reliable player enumeration.

const (
	GVASessionPlayers   uint32 = 0x83691880 // VERIFIED (but unstable across game sessions)
	SessionPlayerStride uint32 = 0x10C      // VERIFIED
	MaxPlayers                 = 16         // VERIFIED
)

// Session player sub-offsets (relative to player struct base).
const (
	SessOffName           uint32 = 0x40 // VERIFIED — UTF-16LE, 32 bytes (16 chars)
	SessOffPrimaryColor   uint32 = 0xA0 // UNVERIFIED — from HaloCaster
	SessOffTeamIndex      uint32 = 0x04 // UNVERIFIED — reads 0 in FFA, untested in team games
	SessOffDisplayedSkill uint32 = 0xA4 // UNVERIFIED — from HaloCaster
)

// ---------------------------------------------------------------------------
// Variant / map info
// ---------------------------------------------------------------------------
//
// Small struct at a fixed GVA containing gametype and scenario path.
// No UTF-16LE variant name was found at this location; readVariantName()
// uses game_results_globals instead.

const (
	GVAVariantInfo uint32 = 0x83606B50 // VERIFIED
)

// Variant sub-offsets (relative to GVAVariantInfo).
const (
	VarOffName     uint32 = 0x00 // Not a real name here — reads pointer bytes, filtered out
	VarOffGameType uint32 = 0x14 // VERIFIED — u8 (observed: 2 = slayer)
	VarOffScenario uint32 = 0x18 // VERIFIED — ASCII path, e.g. "scenarios\multi\lockout\lockout"
)

// ---------------------------------------------------------------------------
// Game results globals
// ---------------------------------------------------------------------------
//
// Monolithic structure containing variant info, session player copies,
// game stats, medal stats, weapon stats, and event buffer.
// Verified via correlated stats scan across host and client instances.
// Replicated identically on host and all clients.

const (
	GVAGameResultsGlobals uint32 = 0x8362AFB0 // VERIFIED — found via correlated u16 stats scan

	// Session player copies within game_results_globals.
	// Uses the SAME index space as game stats (unlike the runtime session
	// player array at GVASessionPlayers, which may use different ordering
	// and can be relocated between games).
	GRGSessionPlayersOff uint32 = 0x0394 // VERIFIED — names found at correct stride
	GRGSessionStride     uint32 = 0xA4   // VERIFIED — confirmed by name spacing in dumps
	GRGSessOffName       uint32 = 0x00   // VERIFIED — UTF-16LE, 32 bytes (16 chars)

	// Variant info within game_results_globals.
	GRGVariantOff     uint32 = 0x013C // VERIFIED — reads "Team Slayer" correctly
	GRGVarNameOff     uint32 = 0x00   // VERIFIED — UTF-16LE variant name
	GRGVarGameTypeOff uint32 = 0x40   // UNVERIFIED — from HaloCaster, used in ReadGameState
	GRGVarScenarioOff uint32 = 0x130  // UNVERIFIED — from HaloCaster, not currently used
)

// ---------------------------------------------------------------------------
// Game event buffer (kill feed) — UNVERIFIED / NON-FUNCTIONAL
// ---------------------------------------------------------------------------
//
// All event buffer constants are from HaloCaster and have NOT been verified
// in xemu. Event count reads 0 despite kills occurring — events may use a
// different mechanism in Halo 2 Xbox, or these offsets may be wrong.
// Kill detection currently relies on stat-diff in events.go instead.

const (
	GVAEventCount       uint32 = 0x8362AFA4 // UNVERIFIED — always reads 0
	GVAEventBuffer      uint32 = 0x8362FF34 // UNVERIFIED — circular buffer base
	EventStructSize     uint32 = 36         // UNVERIFIED
	EventBufferCapacity        = 1000       // UNVERIFIED
)

// Event struct layout (36 bytes per event) — all UNVERIFIED.
const (
	EvtOffType      = 0  // u8: 0=none, 1=kill, 2=carry, 3=score
	EvtOffSource    = 1  // u8: source player index
	EvtOffEffected  = 2  // u8: effected player index
	EvtOffWeapon    = 28 // u32: DamageReportingType enum (kill events)
	EvtOffCarryType = 20 // u32: 1=flag, 2=bomb, 3=oddball (carry events)
	EvtOffTimestamp = 32 // i32: seconds since game start
)

// ---------------------------------------------------------------------------
// Game stats
// ---------------------------------------------------------------------------
//
// Per-player stats within game_results_globals. Players 0-4 at GVAGameStats,
// players 5-15 at GVAGameStatsExtra (stride 0x36A per player).
// Deaths field = total deaths (killed by others + suicides).
// Player index is by join order, not session player slot.
//
// VERIFIED via correlated u16 pattern scan: tres(5k/2d/1s) and dos(1k/8d/3s)
// found at stride 0x36A on all three instances (test1/test2/test3 dumps).

const (
	GVAGameStats      uint32 = 0x8362BF02 // VERIFIED — game_results_globals + 0x0F52
	GVAGameStatsExtra uint32 = 0x8364D014 // VERIFIED — game_results_globals + 0x22064 (players 5-15)
	StatsStride       uint32 = 0x36A      // VERIFIED — confirmed by correlated scan

	OffWeaponStatsInStride uint32 = 0xDE // UNVERIFIED — from HaloCaster
	OffMedalStatsInStride  uint32 = 0x4C // UNVERIFIED — from HaloCaster, not used
)

// Game stats sub-offsets (first 0x0E bytes of stride).
const (
	GSOffKills     uint32 = 0x00 // VERIFIED — u16
	GSOffAssists   uint32 = 0x02 // VERIFIED — u16
	GSOffDeaths    uint32 = 0x04 // VERIFIED — u16 (total: killed + suicides)
	GSOffBetrayals uint32 = 0x06 // VERIFIED — u16
	GSOffSuicides  uint32 = 0x08 // VERIFIED — u16
	GSOffBestSpree uint32 = 0x0A // VERIFIED — u16
	GSOffTimeAlive uint32 = 0x0C // UNVERIFIED — from HaloCaster, not read by any code
)

// Weapon stat sub-offsets (0x10 bytes per weapon) — all UNVERIFIED (from HaloCaster).
const (
	WepOffKills      uint32 = 0x00 // u16
	WepOffShotsFired uint32 = 0x08 // u16
	WepOffShotsHit   uint32 = 0x0A // u16
	WepOffHeadshots  uint32 = 0x0C // u16
	WeaponStatSize   uint32 = 0x10
	WeaponCount             = 41
)
