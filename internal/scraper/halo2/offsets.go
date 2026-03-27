package halo2

// Halo 2 Xbox XBE v1.5 memory layout.
//
// Two address sections with different XBE base offsets:
//   Section A (XBE base 0x59000): session players, variant info, event buffer
//   Section B (XBE base 0x59000 + 0x58000): game stats, weapon stats, medal stats
//
// All data is accessed via constructed high GVAs (>= 0x80000000) so that the
// existing pread-based Mem reader can address them without low-GVA translation.
// Formula: highGVA = 0x80000000 + xbeOffset + haloCasterOffset
//
// Source: HaloCaster (confirmed for this XBE build), StatsBorg, xbox7887.

const (
	// SectionABase = 0x80000000 + 0x59000. Used for session/variant/event data.
	SectionABase uint32 = 0x80059000

	// SectionBBase = 0x80000000 + 0x59000 + 0x58000. Used for kernel-space
	// live game stats (HaloCaster offsets that need the section B shift).
	SectionBBase uint32 = 0x800B1000
)

// ---------------------------------------------------------------------------
// Section A: Session/Variant data (HaloCaster offsets, no shift)
// ---------------------------------------------------------------------------

const (
	// OffVariantInfo: game variant struct.
	//   +0x00: variant name (16 UTF-16LE chars)
	//   +0x40: gametype (u8)
	//   +0x130: scenario path (ASCII, last component = map name)
	OffVariantInfo uint32 = 0x35AD0EC

	// OffSessionPlayers: per-player session properties, stride 0xA4.
	OffSessionPlayers   uint32 = 0x35AD344
	SessionPlayerStride uint32 = 0xA4
)

// Variant sub-offsets (relative to OffVariantInfo).
const (
	VarOffName     uint32 = 0x00
	VarOffGameType uint32 = 0x40
	VarOffScenario uint32 = 0x130
)

// Session player sub-offsets (relative to player base within OffSessionPlayers).
const (
	SessOffName           uint32 = 0x00  // UTF-16LE, 32 bytes (16 chars)
	SessOffPrimaryColor   uint32 = 0x40  // u8
	SessOffTeamIndex      uint32 = 0x7C  // u8
	SessOffDisplayedSkill uint32 = 0x7E  // u8
)

// ---------------------------------------------------------------------------
// Game event buffer (kill feed)
// Source: HaloCaster game_event_monitor.cs
// ---------------------------------------------------------------------------

const (
	OffEventCount       uint32 = 0x35ACFA4 // u32 event counter
	OffEventBuffer      uint32 = 0x35D1F34 // circular buffer base
	EventStructSize     uint32 = 36
	EventBufferCapacity        = 1000
)

// Event struct layout (36 bytes per event).
const (
	EvtOffType      = 0  // u8: 0=none, 1=kill, 2=carry, 3=score
	EvtOffSource    = 1  // u8: source player index
	EvtOffEffected  = 2  // u8: effected player index
	EvtOffWeapon    = 28 // u32: DamageReportingType enum (kill events)
	EvtOffCarryType = 20 // u32: 1=flag, 2=bomb, 3=oddball (carry events)
	EvtOffTimestamp = 32 // i32: seconds since game start
)

// ---------------------------------------------------------------------------
// Section B: Kernel-space live game data
// ---------------------------------------------------------------------------

const (
	// OffGameStats: per-player in-game statistics (raw HaloCaster offset).
	// Struct: s_game_stats (0x36 bytes), stride 0x36A per player.
	OffGameStats uint32 = 0x35ADF02

	// OffGameResultsExtra: overflow for players 5-15.
	OffGameResultsExtra uint32 = 0x35CF014

	// StatsStride is the byte stride between players in game_stats region.
	StatsStride uint32 = 0x36A

	// OffWeaponStatsInStride: weapon stats within the 0x36A stride block.
	// 41 weapons x 0x10 bytes each.
	OffWeaponStatsInStride uint32 = 0xDE

	// OffMedalStatsInStride: medal counts within the 0x36A stride block.
	// 24 medals x 2 bytes each = 0x30 bytes.
	OffMedalStatsInStride uint32 = 0x4C
)

// Game stats sub-offsets (s_game_stats, first 0x36 bytes of stride).
const (
	GSOffKills     uint32 = 0x00 // u16
	GSOffAssists   uint32 = 0x02 // u16
	GSOffDeaths    uint32 = 0x04 // u16
	GSOffBetrayals uint32 = 0x06 // u16
	GSOffSuicides  uint32 = 0x08 // u16
	GSOffBestSpree uint32 = 0x0A // u16
	GSOffTimeAlive uint32 = 0x0C // u16
)

// Weapon stat sub-offsets (s_weapon_stat, 0x10 bytes per weapon).
const (
	WepOffKills      uint32 = 0x00 // u16
	WepOffShotsFired uint32 = 0x08 // u16
	WepOffShotsHit   uint32 = 0x0A // u16
	WepOffHeadshots  uint32 = 0x0C // u16
	WeaponStatSize   uint32 = 0x10
	WeaponCount             = 41
)

// ---------------------------------------------------------------------------
// Pre-computed high GVAs for direct use with Mem.ReadXxx()
// ---------------------------------------------------------------------------

const (
	GVASessionPlayers = SectionABase + OffSessionPlayers   // 0x83606344
	GVAVariantInfo    = SectionABase + OffVariantInfo       // 0x836060EC
	GVAEventCount     = SectionABase + OffEventCount        // 0x836058A4
	GVAEventBuffer    = SectionABase + OffEventBuffer       // 0x8362AF34
	GVAGameStats      = SectionBBase + OffGameStats         // 0x8365EF02
	GVAGameStatsExtra = SectionBBase + OffGameResultsExtra  // 0x83680014
)

const MaxPlayers = 16
