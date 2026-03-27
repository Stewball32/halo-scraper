package haloce

// All addresses are Halo CE guest virtual addresses (GVAs).

// Low guest VAs (< 0x80000000) — require gva2gpa + gpa2hva translation at startup.
// Pointer addresses: read u32 at these to get the actual struct address (>= 0x80000000).
const (
	AddrPlayerDatumArrayPtr   uint32 = 0x2FAD28
	AddrPlayersGlobalsPtr     uint32 = 0x2FAD20
	AddrTeamsPtr              uint32 = 0x2FAD24
	AddrGameGlobalsPtr        uint32 = 0x27629C
	AddrGlobalGameGlobalsPtr  uint32 = 0x39BE4C
	AddrGameServerPtr         uint32 = 0x2E3628
	AddrGameClientPtr         uint32 = 0x2E362C
	AddrObjectHeaderDatumPtr  uint32 = 0x2FC6AC
	AddrGameTimeGlobalsPtr    uint32 = 0x2F8CA0
	AddrGlobalTagInstancesPtr uint32 = 0x39CE24
	AddrGlobalScenarioPtr     uint32 = 0x39BE5C
	AddrGameEngineGlobalsPtr  uint32 = 0x2F9110
	AddrGameVariantGlobalPtr  uint32 = 0x2FAB60
)

// Low guest VAs — direct values (read the value at this address, no further deref).
const (
	AddrGameConnection     uint32 = 0x2E3684 // u16: 0=menu/SP, 1=syslink, 2=hosting, 3=film
	AddrIsTeamGame         uint32 = 0x2F90C4 // u8
	AddrMainMenuActive     uint32 = 0x2E4068 // u8
	AddrGameCanScore       uint32 = 0x2FABF0 // u32: 0=can score, non-zero=game over
	AddrMultiplayerMapName uint32 = 0x2E37CD // null-terminated ASCII
	AddrGlobalStageName    uint32 = 0x2FAC20 // null-terminated ASCII (host only)
	AddrVariant            uint32 = 0x2F90F4 // u8 variant/mode index
)

// Score base addresses by gametype (low GVAs, direct values).
const (
	AddrScoreCTF         uint32 = 0x2762B4 // u32[2] red/blue
	AddrScoreSlayer      uint32 = 0x276710 // u32[16] FFA or u32[2] team
	AddrScoreOddball     uint32 = 0x27653C
	AddrScoreKing        uint32 = 0x2762D8
	AddrScoreRace        uint32 = 0x2766C8
	AddrScoreLimitCTF    uint32 = 0x2762BC // u32
	AddrScoreLimitSlayer uint32 = 0x2F90E8 // u32
	AddrScoreLimitOddball uint32 = 0x276538 // u32
)

// AllLowGVAs is the complete list of low guest VAs that must be translated at
// Instance.Init time. Pass this slice to inst.Init().
var AllLowGVAs = []uint32{
	// Pointer globals
	AddrPlayerDatumArrayPtr,
	AddrPlayersGlobalsPtr,
	AddrTeamsPtr,
	AddrGameGlobalsPtr,
	AddrGlobalGameGlobalsPtr,
	AddrGameServerPtr,
	AddrGameClientPtr,
	AddrObjectHeaderDatumPtr,
	AddrGameTimeGlobalsPtr,
	AddrGlobalTagInstancesPtr,
	AddrGlobalScenarioPtr,
	AddrGameEngineGlobalsPtr,
	AddrGameVariantGlobalPtr,
	// Direct value globals
	AddrGameConnection,
	AddrIsTeamGame,
	AddrMainMenuActive,
	AddrGameCanScore,
	AddrMultiplayerMapName,
	AddrGlobalStageName,
	AddrVariant,
	// Score bases
	AddrScoreCTF,
	AddrScoreSlayer,
	AddrScoreOddball,
	AddrScoreKing,
	AddrScoreRace,
	AddrScoreLimitCTF,
	AddrScoreLimitSlayer,
	AddrScoreLimitOddball,
}

// Offsets within GameTimeGlobals struct (at *AddrGameTimeGlobalsPtr).
const (
	OffGTGInitialized uint32 = 0x00 // u8
	OffGTGActive      uint32 = 0x01 // u8
	OffGTGPaused      uint32 = 0x02 // u8
	OffGTGGameTime    uint32 = 0x0C // u32 ticks (30Hz)
	OffGTGElapsed     uint32 = 0x10 // u32
	OffGTGSpeed       uint32 = 0x18 // f32
)

// Offsets within GameEngineGlobals struct (at *AddrGameEngineGlobalsPtr).
// Pointer is 0 in pregame lobby.
const (
	OffGEGGametype uint32 = 0x04 // u32 gametype ID (1–5)
)

// Offsets within PlayerDatumArray header (at *AddrPlayerDatumArrayPtr).
const (
	OffPDAMaxCount     uint32 = 0x20 // u16
	OffPDAElementSize  uint32 = 0x22 // u16 (typically 0xD4 = 212)
	OffPDACurrentCount uint32 = 0x2E // u16
	OffPDAFirstElement uint32 = 0x34 // u32
)

// Offsets within static player struct (at firstElement + index * elementSize).
const (
	OffPlrLocalIndex   uint32 = 0x02 // s16: -1 if remote
	OffPlrName         uint32 = 0x04 // [24] UTF-16LE, 12 chars
	OffPlrTeam         uint32 = 0x20 // u32
	OffPlrRespawnTimer uint32 = 0x2C // u32 countdown ticks until next spawn
	OffPlrObjectHandle uint32 = 0x34 // s32: -1 when dead
	OffPlrPrevObjHandle uint32 = 0x38 // s32: previous object handle (for death tick reads)
	OffPlrKillStreak   uint32 = 0x92 // u16: resets to 0 on death
	OffPlrMultikill    uint32 = 0x94 // u16: resets to 0 on death
	OffPlrTimeLastKill uint32 = 0x96 // s16: game ticks; -1 on death
	OffPlrKills        uint32 = 0x98 // s16
	OffPlrAssists      uint32 = 0xA0 // s16
	OffPlrTeamKills    uint32 = 0xA8 // s16
	OffPlrDeaths       uint32 = 0xAA // s16
	OffPlrSuicides     uint32 = 0xAC // s16
	OffPlrShotsFired   uint32 = 0xAE // s32
	OffPlrShotsHit     uint32 = 0xB2 // s16
	OffPlrCTFScore     uint32 = 0xC4 // s16
	OffPlrQuit         uint32 = 0xD1 // u8: 1 = player quit
)

// Offsets within ObjectHeaderDatumArray header (at *AddrObjectHeaderDatumPtr).
const (
	OffOHDMaxElements  uint32 = 0x20 // u16
	OffOHDElementSize  uint32 = 0x22 // u16 (typically 12)
	OffOHDAllocCount   uint32 = 0x2E // u16: iterate this many entries
	OffOHDElementCount uint32 = 0x30 // u16
	OffOHDFirstElement uint32 = 0x34 // u32: re-read every tick
)

// Offsets within object header entry (stride = element_size = 12).
const (
	OffObjEntryDataAddr uint32 = 0x08 // u32: object_data_addr (0 if slot empty)
)

// Common object data offsets (at object_data_addr, all object types).
const (
	OffObjTagIndex  uint32 = 0x00 // s16: tag index (low 16 bits)
	OffObjFlags     uint32 = 0x04 // u32: &0x10000=garbage
	OffObjX         uint32 = 0x0C // f32
	OffObjY         uint32 = 0x10 // f32
	OffObjZ         uint32 = 0x14 // f32
	OffObjType      uint32 = 0x64 // u8: 0=biped,1=vehicle,2=weapon,3=equipment,...
	ObjFlagGarbage  uint32 = 0x10000
)

// Offsets within dynamic player / biped object (at object_data_addr, type=0).
const (
	OffDynX             uint32 = 0x0C  // f32
	OffDynY             uint32 = 0x10  // f32
	OffDynZ             uint32 = 0x14  // f32
	OffDynVelX          uint32 = 0x18  // f32
	OffDynVelY          uint32 = 0x1C  // f32
	OffDynVelZ          uint32 = 0x20  // f32
	OffDynParentObject  uint32 = 0xCC  // u32: vehicle handle; 0xFFFFFFFF=on foot
	OffDynMaxHealth     uint32 = 0x88  // f32
	OffDynMaxShields    uint32 = 0x8C  // f32
	OffDynHealth        uint32 = 0x90  // f32
	OffDynShields       uint32 = 0x94  // f32
	OffDynShieldsStatus uint32 = 0xB6  // u16
	OffDynCurrentAction uint32 = 0x1B8 // u32 bitfield
	OffDynCamo          uint32 = 0x1B4 // u8: 0x41=no camo, 0x51=active camo
	OffDynAimX          uint32 = 0x1EC // f32 aiming_vector x
	OffDynAimY          uint32 = 0x1F0 // f32 aiming_vector y
	OffDynAimZ          uint32 = 0x1F4 // f32 aiming_vector z
	OffDynSelectedSlot  uint32 = 0x2A2 // s16: 0=primary, 1=secondary, -1=none
	OffDynWeaponSlot0   uint32 = 0x2A8 // u32 handle
	OffDynWeaponSlot1   uint32 = 0x2AC // u32 handle
	OffDynWeaponSlot2   uint32 = 0x2B0 // u32 handle
	OffDynWeaponSlot3   uint32 = 0x2B4 // u32 handle
	OffDynFrags         uint32 = 0x2CE // u8
	OffDynPlasmas       uint32 = 0x2CF // u8
	OffDynZoomLevel     uint32 = 0x2D0 // s8 (read as u8, cast to int8)
	OffDynCrouchScale   uint32 = 0x464 // f32: 0.0=standing, 1.0=crouching
	OffDynCamoAmount    uint32 = 0x32C // f32
	OffDynMeleeRemaining  uint32 = 0x45D // u8
	OffDynMeleeDamageTick uint32 = 0x45E // u8: equals 0x45D when melee impacts
	OffDynDamageTable   uint32 = 0x3E0 // 4-slot damage history
)

// current_action bitfield masks (OffDynCurrentAction).
const (
	ActionCrouch      uint32 = 0x0001
	ActionJump        uint32 = 0x0002
	ActionFire        uint32 = 0x0008
	ActionFlashlight  uint32 = 0x0010
	ActionPressAction uint32 = 0x0440
	ActionShooting    uint32 = 0x0800
	ActionGrenade     uint32 = 0x2FC4
	ActionHoldAction  uint32 = 0x4000
)

// Damage table (at dynPlayerAddr + OffDynDamageTable), 4 entries × 16 bytes.
const (
	DamageTableSlots    = 4
	DamageEntrySize     = 16
	OffDmgTime          uint32 = 0x00 // u32 game tick; 0xFFFFFFFF=empty
	OffDmgAmount        uint32 = 0x04 // f32
	OffDmgDealerObjHdl  uint32 = 0x08 // u32 dynamic object handle
	OffDmgDealerPlrHdl  uint32 = 0x0C // u32 static player handle (&0xFFFF=index)
)

// Weapon object offsets (at object_data_addr, type=2).
const (
	OffWepAmmoMag   uint32 = 0x260 // s16 current magazine
	OffWepAmmoPack  uint32 = 0x25E // s16 reserve ammo
	OffWepCharge    uint32 = 0x0F0 // f32 energy remaining (0–1)
	OffWepEnergyUsed uint32 = 0x1F0 // f32: 1.0 = depleted energy weapon
)

// Weapon tag data offsets (at tag_data_ptr, accessed via tag instance array).
const (
	OffWepTagWeaponType uint32 = 0x309 // u8: &0x8 = energy weapon
)

// Tag instance array (at *AddrGlobalTagInstancesPtr, stride 32 bytes per entry).
const (
	TagInstStride  = 32
	OffTagNamePtr  uint32 = 0x10 // u32 → null-terminated tag name string
	OffTagDataPtr  uint32 = 0x14 // u32 → raw tag data struct
)

// Scenario item spawn offsets (at *AddrGlobalScenarioPtr).
const (
	OffScenarioItemCount uint32 = 900 // s32
	OffScenarioItemFirst uint32 = 904 // u32
	ScenarioItemStride          = 144 // bytes per entry
	OffScenItemTagIndex  uint32 = 0x5C // s32 tag index (-1 if empty)
	OffScenItemX         uint32 = 0x40 // f32
	OffScenItemY         uint32 = 0x44 // f32
	OffScenItemZ         uint32 = 0x48 // f32
	// Spawn interval: read_u32(tagDataPtr + OffTagDataPtr) → base; read_s16(base + 0x0C)
	OffTagRespawnIntervalOff uint32 = 0x14 // u32 at tag_data → pointer to interval table
	OffTagRespawnInterval    uint32 = 0x0C // s16 within interval table
)
