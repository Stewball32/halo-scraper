package halo

import (
	"encoding/binary"
	"math"
	"unicode/utf16"

	"xemu-cartographer/internal/xemu"
)

// Reader reads Halo CE game state from a single xemu instance.
type Reader struct {
	inst         *xemu.Instance
	name         string
	tagNameCache map[int16]string
	tagInstBase  uint32 // cached; 0 = not yet read
	ohdBase      uint32 // cached; 0 = not yet read
}

// NewReader creates a Reader for the given instance.
// inst.Init(AllLowGVAs) must have been called before use.
func NewReader(inst *xemu.Instance, instanceName string) *Reader {
	return &Reader{
		inst:         inst,
		name:         instanceName,
		tagNameCache: make(map[int16]string),
	}
}

// -------------------------------------------------------------------
// Game state (lightweight — called every iteration of the poll loop)
// -------------------------------------------------------------------

// ReadGameState reads the minimum needed to determine game state and current tick.
func (r *Reader) ReadGameState() (state GameState, tick uint32, err error) {
	inst := r.inst
	mem := inst.Mem

	geGlobalsPtr, err := inst.DerefLowPtr(AddrGameEngineGlobalsPtr)
	if err != nil {
		return GameStateMenu, 0, err
	}
	gameEngineRunning := geGlobalsPtr != 0

	mainMenuHVA, err := inst.LowHVA(AddrMainMenuActive)
	if err != nil {
		return GameStateMenu, 0, err
	}
	mainMenu, err := mem.ReadU8At(mainMenuHVA)
	if err != nil {
		return GameStateMenu, 0, err
	}

	gtgPtr, err := inst.DerefLowPtr(AddrGameTimeGlobalsPtr)
	if err != nil {
		return GameStateMenu, 0, err
	}

	var initialized, active, paused uint8
	if gtgPtr >= 0x80000000 {
		initialized, _ = mem.ReadU8(gtgPtr + OffGTGInitialized)
		active, _ = mem.ReadU8(gtgPtr + OffGTGActive)
		paused, _ = mem.ReadU8(gtgPtr + OffGTGPaused)
		tick, _ = mem.ReadU32(gtgPtr + OffGTGGameTime)
	}

	gameCanScoreHVA, err := inst.LowHVA(AddrGameCanScore)
	if err != nil {
		return GameStateMenu, tick, err
	}
	gameCanScore, _ := mem.ReadU32At(gameCanScoreHVA)

	state = determineGameState(mainMenu, initialized, active, paused, gameEngineRunning, gameCanScore)
	return state, tick, nil
}

func determineGameState(mainMenu, initialized, active, paused uint8, engineRunning bool, gameCanScore uint32) GameState {
	if mainMenu != 0 || initialized == 0 {
		return GameStateMenu
	}
	if initialized == 1 && active == 0 && paused == 1 {
		return GameStatePreGame
	}
	if initialized == 1 && active == 1 && paused == 0 {
		if engineRunning && gameCanScore != 0 {
			return GameStatePostGame
		}
		return GameStateInGame
	}
	return GameStateMenu
}

// -------------------------------------------------------------------
// Snapshot (called on game-state transition / client connect)
// -------------------------------------------------------------------

// ReadSnapshot reads the full static game state.
func (r *Reader) ReadSnapshot() (SnapshotPayload, error) {
	inst := r.inst
	mem := inst.Mem

	mapName := r.readLowString(AddrMultiplayerMapName, 32)
	isTeamGameHVA, _ := inst.LowHVA(AddrIsTeamGame)
	isTeamGameV, _ := mem.ReadU8At(isTeamGameHVA)
	isTeamGame := isTeamGameV != 0

	gametypeID, err := r.readGametypeID()
	if err != nil {
		gametypeID = 0
	}
	gametypeName := GametypeNames[gametypeID]
	if gametypeName == "" {
		gametypeName = "unknown"
	}

	scoreLimit, _ := r.readScoreLimit(gametypeID)
	teamScores, _ := r.readTeamScores(gametypeID, isTeamGame)
	players, _ := r.readSnapshotPlayers()
	spawns, _ := r.readPowerItemSpawns()

	return SnapshotPayload{
		Map:             mapName,
		Gametype:        gametypeName,
		IsTeamGame:      isTeamGame,
		ScoreLimit:      scoreLimit,
		TimeLimitTicks:  0, // no verified address
		TeamScores:      teamScores,
		Players:         players,
		PowerItemSpawns: spawns,
	}, nil
}

func (r *Reader) readGametypeID() (uint32, error) {
	gePtr, err := r.inst.DerefLowPtr(AddrGameEngineGlobalsPtr)
	if err != nil || gePtr < 0x80000000 {
		return 0, err
	}
	return r.inst.Mem.ReadU32(gePtr + OffGEGGametype)
}

func (r *Reader) readScoreLimit(gametypeID uint32) (int32, error) {
	var hva int64
	var err error
	switch gametypeID {
	case 1:
		hva, err = r.inst.LowHVA(AddrScoreLimitCTF)
	case 2:
		hva, err = r.inst.LowHVA(AddrScoreLimitSlayer)
	case 3:
		hva, err = r.inst.LowHVA(AddrScoreLimitOddball)
	default:
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	v, err := r.inst.Mem.ReadU32At(hva)
	return int32(v), err
}

func (r *Reader) readTeamScores(gametypeID uint32, isTeamGame bool) ([]TeamScore, error) {
	if !isTeamGame {
		return nil, nil
	}
	var addrLow uint32
	switch gametypeID {
	case 1:
		addrLow = AddrScoreCTF
	case 2:
		addrLow = AddrScoreSlayer
	case 3:
		addrLow = AddrScoreOddball
	case 4:
		addrLow = AddrScoreKing
	default:
		return nil, nil
	}
	hva, err := r.inst.LowHVA(addrLow)
	if err != nil {
		return nil, err
	}
	red, err := r.inst.Mem.ReadU32At(hva)
	if err != nil {
		return nil, err
	}
	blue, err := r.inst.Mem.ReadU32At(hva + 4)
	if err != nil {
		return nil, err
	}
	return []TeamScore{
		{Team: 0, Score: int32(red)},
		{Team: 1, Score: int32(blue)},
	}, nil
}

func (r *Reader) readSnapshotPlayers() ([]SnapshotPlayer, error) {
	inst := r.inst
	mem := inst.Mem

	pdaBase, err := inst.DerefLowPtr(AddrPlayerDatumArrayPtr)
	if err != nil || pdaBase < 0x80000000 {
		return nil, err
	}
	elemSize, _ := mem.ReadU16(pdaBase + OffPDAElementSize)
	currentCount, _ := mem.ReadU16(pdaBase + OffPDACurrentCount)
	firstElement, _ := mem.ReadU32(pdaBase + OffPDAFirstElement)
	if firstElement < 0x80000000 || currentCount == 0 || elemSize == 0 {
		return nil, nil
	}

	players := make([]SnapshotPlayer, 0, currentCount)
	for i := uint16(0); i < currentCount; i++ {
		base := firstElement + uint32(i)*uint32(elemSize)
		p, ok, err := r.readSnapshotPlayer(int(i), base)
		if err != nil || !ok {
			continue
		}
		players = append(players, p)
	}
	return players, nil
}

func (r *Reader) readSnapshotPlayer(index int, base uint32) (SnapshotPlayer, bool, error) {
	mem := r.inst.Mem

	nameBytes, err := mem.ReadBytes(base+OffPlrName, 24)
	if err != nil {
		return SnapshotPlayer{}, false, err
	}
	if nameBytes[0] == 0 && nameBytes[1] == 0 {
		return SnapshotPlayer{}, false, nil
	}

	team, _ := mem.ReadU32(base + OffPlrTeam)
	kills, _ := mem.ReadS16(base + OffPlrKills)
	deaths, _ := mem.ReadS16(base + OffPlrDeaths)
	assists, _ := mem.ReadS16(base + OffPlrAssists)
	teamKills, _ := mem.ReadS16(base + OffPlrTeamKills)
	suicides, _ := mem.ReadS16(base + OffPlrSuicides)
	ctfScore, _ := mem.ReadS16(base + OffPlrCTFScore)
	ksRaw, _ := mem.ReadU16(base + OffPlrKillStreak)
	mkRaw, _ := mem.ReadU16(base + OffPlrMultikill)
	sfRaw, _ := mem.ReadS32(base + OffPlrShotsFired)
	shRaw, _ := mem.ReadS16(base + OffPlrShotsHit)

	return SnapshotPlayer{
		Index:      index,
		Name:       decodeUTF16LE(nameBytes),
		Team:       team,
		Kills:      kills,
		Deaths:     deaths,
		Assists:    assists,
		CTFScore:   ctfScore,
		TeamKills:  teamKills,
		Suicides:   suicides,
		KillStreak: ksRaw,
		Multikill:  mkRaw,
		ShotsFired: sfRaw,
		ShotsHit:   shRaw,
	}, true, nil
}

// -------------------------------------------------------------------
// Tick (called every game tick when in_game)
// -------------------------------------------------------------------

// ReadTick reads all 30Hz dynamic state for one tick.
func (r *Reader) ReadTick(spawns []PowerItemSpawn, state *TickState) (TickResult, error) {
	inst := r.inst
	mem := inst.Mem

	// Ensure cached bases are populated.
	if err := r.ensureBases(); err != nil {
		return TickResult{}, err
	}

	// Re-read object header first_element_address every tick (table rearranges every ~30s).
	var objHeaderFirst uint32
	var objElemSize uint16
	var objAllocCount uint16
	if r.ohdBase >= 0x80000000 {
		objElemSize, _ = mem.ReadU16(r.ohdBase + OffOHDElementSize)
		objHeaderFirst, _ = mem.ReadU32(r.ohdBase + OffOHDFirstElement)
		objAllocCount, _ = mem.ReadU16(r.ohdBase + OffOHDAllocCount)
	}

	// Read player datum array.
	pdaBase, err := inst.DerefLowPtr(AddrPlayerDatumArrayPtr)
	if err != nil || pdaBase < 0x80000000 {
		return TickResult{}, err
	}
	elemSize, _ := mem.ReadU16(pdaBase + OffPDAElementSize)
	currentCount, _ := mem.ReadU16(pdaBase + OffPDACurrentCount)
	firstElement, _ := mem.ReadU32(pdaBase + OffPDAFirstElement)
	if firstElement < 0x80000000 || elemSize == 0 {
		return TickResult{}, nil
	}

	tickPlayers := make([]TickPlayer, 0, currentCount)
	internalPlayers := make([]InternalPlayerState, 0, currentCount)

	for i := uint16(0); i < currentCount; i++ {
		playerBase := firstElement + uint32(i)*uint32(elemSize)
		tp, ip, ok, err := r.readTickPlayer(int(i), playerBase, objHeaderFirst, objElemSize)
		if err != nil || !ok {
			continue
		}
		tickPlayers = append(tickPlayers, tp)
		internalPlayers = append(internalPlayers, ip)
	}

	// Build playerSlots map: objectID → player index (for power item held detection).
	playerSlots := make(map[uint32]int, len(internalPlayers)*4)
	for _, ip := range internalPlayers {
		for _, handle := range ip.WeaponSlots {
			if handle != 0xFFFFFFFF {
				oid := handle & 0xFFFF
				playerSlots[oid] = ip.Index
			}
		}
	}

	powerItems := r.readPowerItemStatus(spawns, state, playerSlots, objHeaderFirst, objElemSize, objAllocCount)

	result := TickResult{
		Payload: TickPayload{
			Players:    tickPlayers,
			PowerItems: powerItems,
		},
		InternalPlayers: internalPlayers,
	}
	return result, nil
}

func (r *Reader) readTickPlayer(
	index int,
	playerBase uint32,
	objHeaderFirst uint32,
	objElemSize uint16,
) (TickPlayer, InternalPlayerState, bool, error) {
	mem := r.inst.Mem

	// Active slot check: name's first UTF-16 char must be non-zero.
	nameBytes, err := mem.ReadBytes(playerBase+OffPlrName, 2)
	if err != nil {
		return TickPlayer{}, InternalPlayerState{}, false, err
	}
	if nameBytes[0] == 0 && nameBytes[1] == 0 {
		return TickPlayer{}, InternalPlayerState{}, false, nil
	}

	// Static fields.
	ip := InternalPlayerState{Index: index}
	ip.Kills, _ = mem.ReadS16(playerBase + OffPlrKills)
	ip.Deaths, _ = mem.ReadS16(playerBase + OffPlrDeaths)
	ip.Assists, _ = mem.ReadS16(playerBase + OffPlrAssists)
	ip.TeamKills, _ = mem.ReadS16(playerBase + OffPlrTeamKills)
	ip.Suicides, _ = mem.ReadS16(playerBase + OffPlrSuicides)
	ksRaw, _ := mem.ReadU16(playerBase + OffPlrKillStreak)
	mkRaw, _ := mem.ReadU16(playerBase + OffPlrMultikill)
	ip.KillStreak = ksRaw
	ip.Multikill = mkRaw
	ip.ShotsFired, _ = mem.ReadS32(playerBase + OffPlrShotsFired)
	ip.ShotsHit, _ = mem.ReadS16(playerBase + OffPlrShotsHit)
	quitRaw, _ := mem.ReadU8(playerBase + OffPlrQuit)
	ip.QuitFlag = quitRaw
	respawnRaw, _ := mem.ReadU32(playerBase + OffPlrRespawnTimer)
	ip.RespawnTimer = respawnRaw

	handle, _ := mem.ReadS32(playerBase + OffPlrObjectHandle)
	prevHandle, _ := mem.ReadS32(playerBase + OffPlrPrevObjHandle)

	alive := handle != -1

	tp := TickPlayer{
		Index: index,
		Alive: alive,
	}

	if !alive && respawnRaw > 0 {
		v := respawnRaw
		tp.RespawnInTicks = &v
	}

	// Dynamic object data.
	if alive && objHeaderFirst >= 0x80000000 && objElemSize > 0 {
		objIdx := uint32(handle) & 0xFFFF
		objEntryAddr := objHeaderFirst + objIdx*uint32(objElemSize)
		objDataAddr, _ := mem.ReadU32(objEntryAddr + OffObjEntryDataAddr)
		if objDataAddr >= 0x80000000 {
			ip.ObjDataAddr = objDataAddr
			r.readDynPlayerFull(&tp, &ip, objDataAddr)
		}
	}

	// Previous biped for damage table reads when dead.
	if !alive && prevHandle != -1 && objHeaderFirst >= 0x80000000 && objElemSize > 0 {
		prevIdx := uint32(prevHandle) & 0xFFFF
		prevEntry := objHeaderFirst + prevIdx*uint32(objElemSize)
		prevAddr, _ := mem.ReadU32(prevEntry + OffObjEntryDataAddr)
		if prevAddr >= 0x80000000 {
			ip.PrevObjDataAddr = prevAddr
			// Read damage table from previous biped.
			ip.DamageTable = r.readDamageTable(prevAddr)
		}
	}
	if alive && ip.ObjDataAddr >= 0x80000000 {
		ip.DamageTable = r.readDamageTable(ip.ObjDataAddr)
	}

	return tp, ip, true, nil
}

func (r *Reader) readDynPlayerFull(tp *TickPlayer, ip *InternalPlayerState, objDataAddr uint32) {
	mem := r.inst.Mem

	tp.X, _ = mem.ReadF32(objDataAddr + OffDynX)
	tp.Y, _ = mem.ReadF32(objDataAddr + OffDynY)
	tp.Z, _ = mem.ReadF32(objDataAddr + OffDynZ)
	tp.VX, _ = mem.ReadF32(objDataAddr + OffDynVelX)
	tp.VY, _ = mem.ReadF32(objDataAddr + OffDynVelY)
	tp.VZ, _ = mem.ReadF32(objDataAddr + OffDynVelZ)
	tp.AimX, _ = mem.ReadF32(objDataAddr + OffDynAimX)
	tp.AimY, _ = mem.ReadF32(objDataAddr + OffDynAimY)
	tp.AimZ, _ = mem.ReadF32(objDataAddr + OffDynAimZ)

	zoomRaw, _ := mem.ReadU8(objDataAddr + OffDynZoomLevel)
	tp.ZoomLevel = int8(zoomRaw)
	tp.CrouchScale, _ = mem.ReadF32(objDataAddr + OffDynCrouchScale)
	tp.Health, _ = mem.ReadF32(objDataAddr + OffDynHealth)
	tp.Shields, _ = mem.ReadF32(objDataAddr + OffDynShields)

	shieldsStatus, _ := mem.ReadU16(objDataAddr + OffDynShieldsStatus)
	tp.HasOvershield = (shieldsStatus & 0x10) != 0

	camo, _ := mem.ReadU8(objDataAddr + OffDynCamo)
	tp.HasCamo = camo == 0x51

	fragsRaw, _ := mem.ReadU8(objDataAddr + OffDynFrags)
	plasmasRaw, _ := mem.ReadU8(objDataAddr + OffDynPlasmas)
	tp.Frags = fragsRaw
	tp.Plasmas = plasmasRaw

	selectedSlot, _ := mem.ReadS16(objDataAddr + OffDynSelectedSlot)
	tp.SelectedWeaponSlot = selectedSlot

	action, _ := mem.ReadU32(objDataAddr + OffDynCurrentAction)
	tp.IsCrouching = action&ActionCrouch != 0
	tp.IsJumping = action&ActionJump != 0
	tp.IsFiring = action&ActionFire != 0
	tp.IsShooting = action&ActionShooting != 0
	tp.IsFlashlightOn = action&ActionFlashlight != 0
	tp.IsThrowingGrenade = action&ActionGrenade != 0
	tp.IsPressingAction = action&ActionPressAction != 0
	tp.IsHoldingAction = action&ActionHoldAction != 0

	meleeRem, _ := mem.ReadU8(objDataAddr + OffDynMeleeRemaining)
	meleeDmg, _ := mem.ReadU8(objDataAddr + OffDynMeleeDamageTick)
	ip.MeleeRemaining = meleeRem
	ip.MeleeDamageTick = meleeDmg
	tp.IsMeleeing = meleeRem > 0

	parentObj, _ := mem.ReadU32(objDataAddr + OffDynParentObject)
	ip.ParentObject = parentObj

	// Weapon slots.
	wepOffsets := [4]uint32{OffDynWeaponSlot0, OffDynWeaponSlot1, OffDynWeaponSlot2, OffDynWeaponSlot3}
	for slot, off := range wepOffsets {
		handle, _ := mem.ReadU32(objDataAddr + off)
		ip.WeaponSlots[slot] = handle
		if handle != 0xFFFFFFFF {
			wi, ok, _ := r.readWeaponInfo(slot, handle)
			if ok {
				tp.Weapons = append(tp.Weapons, wi)
			}
		}
	}
}

func (r *Reader) readWeaponInfo(slot int, handle uint32) (WeaponInfo, bool, error) {
	if handle == 0xFFFFFFFF || r.ohdBase < 0x80000000 {
		return WeaponInfo{}, false, nil
	}
	mem := r.inst.Mem

	objElemSize, _ := mem.ReadU16(r.ohdBase + OffOHDElementSize)
	objHeaderFirst, _ := mem.ReadU32(r.ohdBase + OffOHDFirstElement)
	if objHeaderFirst < 0x80000000 || objElemSize == 0 {
		return WeaponInfo{}, false, nil
	}

	objIdx := handle & 0xFFFF
	entryAddr := objHeaderFirst + objIdx*uint32(objElemSize)
	objDataAddr, _ := mem.ReadU32(entryAddr + OffObjEntryDataAddr)
	if objDataAddr < 0x80000000 {
		return WeaponInfo{}, false, nil
	}

	flags, _ := mem.ReadU32(objDataAddr + OffObjFlags)
	if flags&ObjFlagGarbage != 0 {
		return WeaponInfo{}, false, nil
	}

	tagIdx, _ := mem.ReadS16(objDataAddr + OffObjTagIndex)
	tagName, _ := r.readTagName(tagIdx)

	isEnergy := false
	if r.tagInstBase >= 0x80000000 {
		tagInstEntry := r.tagInstBase + uint32(TagInstStride)*uint32(uint16(tagIdx))
		tagDataPtr, _ := mem.ReadU32(tagInstEntry + OffTagDataPtr)
		if tagDataPtr >= 0x80000000 {
			weaponTypeRaw, _ := mem.ReadU8(tagDataPtr + OffWepTagWeaponType)
			isEnergy = weaponTypeRaw&0x08 != 0
		}
	}

	wi := WeaponInfo{
		Slot:     slot,
		ObjectID: handle & 0xFFFF,
		Tag:      tagName,
		IsEnergy: isEnergy,
	}

	if isEnergy {
		charge, _ := mem.ReadF32(objDataAddr + OffWepCharge)
		wi.Charge = &charge
	} else {
		mag, _ := mem.ReadS16(objDataAddr + OffWepAmmoMag)
		pack, _ := mem.ReadS16(objDataAddr + OffWepAmmoPack)
		wi.AmmoMag = &mag
		wi.AmmoPack = &pack
	}

	return wi, true, nil
}

func (r *Reader) readDamageTable(objDataAddr uint32) [DamageTableSlots]DamageEntry {
	mem := r.inst.Mem
	var table [DamageTableSlots]DamageEntry
	base := objDataAddr + OffDynDamageTable
	for i := 0; i < DamageTableSlots; i++ {
		off := base + uint32(i)*DamageEntrySize
		t, _ := mem.ReadU32(off + OffDmgTime)
		a, _ := mem.ReadF32(off + OffDmgAmount)
		doh, _ := mem.ReadU32(off + OffDmgDealerObjHdl)
		dph, _ := mem.ReadU32(off + OffDmgDealerPlrHdl)
		table[i] = DamageEntry{DamageTime: t, Amount: a, DealerObjHandle: doh, DealerPlrHandle: dph}
	}
	return table
}

// -------------------------------------------------------------------
// Power item status
// -------------------------------------------------------------------

func (r *Reader) readPowerItemStatus(
	spawns []PowerItemSpawn,
	state *TickState,
	playerSlots map[uint32]int, // objectID → player index
	objHeaderFirst uint32,
	objElemSize uint16,
	objAllocCount uint16,
) []PowerItemStatus {
	if len(spawns) == 0 {
		return nil
	}

	mem := r.inst.Mem

	// Build world object map: tag → list of (objectID, x, y, z).
	type worldObj struct {
		objectID uint32
		x, y, z  float32
	}
	worldByTag := make(map[string][]worldObj)

	if objHeaderFirst >= 0x80000000 && objElemSize > 0 {
		for i := uint16(0); i < objAllocCount; i++ {
			entryAddr := objHeaderFirst + uint32(i)*uint32(objElemSize)
			objDataAddr, _ := mem.ReadU32(entryAddr + OffObjEntryDataAddr)
			if objDataAddr < 0x80000000 {
				continue
			}
			flags, _ := mem.ReadU32(objDataAddr + OffObjFlags)
			if flags&ObjFlagGarbage != 0 {
				continue
			}
			tagIdx, _ := mem.ReadS16(objDataAddr + OffObjTagIndex)
			tagName, err := r.readTagName(tagIdx)
			if err != nil || tagName == "" {
				continue
			}
			x, _ := mem.ReadF32(objDataAddr + OffObjX)
			y, _ := mem.ReadF32(objDataAddr + OffObjY)
			z, _ := mem.ReadF32(objDataAddr + OffObjZ)
			worldByTag[tagName] = append(worldByTag[tagName], worldObj{uint32(i), x, y, z})
		}
	}

	result := make([]PowerItemStatus, len(spawns))
	for idx, spawn := range spawns {
		tracker, ok := state.PowerItems[spawn.SpawnID]
		if !ok {
			// Tracker not initialised — create one.
			tracker = &PowerItemTracker{CurrentObjectID: 0xFFFF, Status: "respawning", HeldBy: -1}
			state.PowerItems[spawn.SpawnID] = tracker
		}

		status := PowerItemStatus{SpawnID: spawn.SpawnID}

		// Check if tracked object is in a player's weapon slot.
		if tracker.CurrentObjectID != 0xFFFF {
			if playerIdx, held := playerSlots[tracker.CurrentObjectID]; held {
				tracker.Status = "held"
				tracker.HeldBy = playerIdx
				status.Status = "held"
				status.HeldBy = &playerIdx
				result[idx] = status
				continue
			}
		}

		// Check world objects for this spawn's tag.
		worldObjs := worldByTag[spawn.Tag]
		if len(worldObjs) > 0 {
			var best *worldObj
			// Prefer the currently tracked object if still in world.
			for i := range worldObjs {
				if tracker.CurrentObjectID != 0xFFFF && worldObjs[i].objectID == tracker.CurrentObjectID {
					best = &worldObjs[i]
					break
				}
			}
			// Otherwise pick closest to spawn point.
			if best == nil {
				minDist := float32(math.MaxFloat32)
				for i := range worldObjs {
					dx := worldObjs[i].x - spawn.X
					dy := worldObjs[i].y - spawn.Y
					dz := worldObjs[i].z - spawn.Z
					d := dx*dx + dy*dy + dz*dz
					if d < minDist {
						minDist = d
						best = &worldObjs[i]
					}
				}
				// New or re-spawned object found.
				tracker.CurrentObjectID = best.objectID
			}
			tracker.Status = "world"
			tracker.HeldBy = -1
			pos := &XYZ{X: best.x, Y: best.y, Z: best.z}
			status.Status = "world"
			status.WorldPos = pos
			result[idx] = status
			continue
		}

		// Not held, not in world → respawning.
		if tracker.Status == "respawning" {
			tracker.RespawnTimer--
			if tracker.RespawnTimer < 0 {
				tracker.RespawnTimer = 0
			}
		} else {
			// Just transitioned to respawning.
			tracker.Status = "respawning"
			tracker.HeldBy = -1
			tracker.CurrentObjectID = 0xFFFF
			tracker.RespawnTimer = int32(spawn.SpawnIntervalTicks)
		}
		rt := tracker.RespawnTimer
		status.Status = "respawning"
		status.RespawnInTicks = &rt
		result[idx] = status
	}
	return result
}

// -------------------------------------------------------------------
// Power item spawns (read once at game start)
// -------------------------------------------------------------------

func (r *Reader) readPowerItemSpawns() ([]PowerItemSpawn, error) {
	inst := r.inst
	mem := inst.Mem

	scenarioBase, err := inst.DerefLowPtr(AddrGlobalScenarioPtr)
	if err != nil || scenarioBase < 0x80000000 {
		return nil, err
	}

	if err := r.ensureBases(); err != nil {
		return nil, err
	}

	itemCount, _ := mem.ReadS32(scenarioBase + OffScenarioItemCount)
	if itemCount <= 0 {
		return nil, nil
	}
	firstItemAddr, _ := mem.ReadU32(scenarioBase + OffScenarioItemFirst)
	if firstItemAddr < 0x80000000 {
		return nil, nil
	}

	// Build world object index for initial objectID lookup.
	var objHeaderFirst uint32
	var objElemSize uint16
	var objAllocCount uint16
	if r.ohdBase >= 0x80000000 {
		objElemSize, _ = mem.ReadU16(r.ohdBase + OffOHDElementSize)
		objHeaderFirst, _ = mem.ReadU32(r.ohdBase + OffOHDFirstElement)
		objAllocCount, _ = mem.ReadU16(r.ohdBase + OffOHDAllocCount)
	}

	type worldObj struct {
		objectID    uint32
		tagIdx      int16
		x, y, z     float32
	}
	var worldObjs []worldObj
	if objHeaderFirst >= 0x80000000 && objElemSize > 0 {
		for i := uint16(0); i < objAllocCount; i++ {
			entryAddr := objHeaderFirst + uint32(i)*uint32(objElemSize)
			objDataAddr, _ := mem.ReadU32(entryAddr + OffObjEntryDataAddr)
			if objDataAddr < 0x80000000 {
				continue
			}
			flags, _ := mem.ReadU32(objDataAddr + OffObjFlags)
			if flags&ObjFlagGarbage != 0 {
				continue
			}
			tIdx, _ := mem.ReadS16(objDataAddr + OffObjTagIndex)
			x, _ := mem.ReadF32(objDataAddr + OffObjX)
			y, _ := mem.ReadF32(objDataAddr + OffObjY)
			z, _ := mem.ReadF32(objDataAddr + OffObjZ)
			worldObjs = append(worldObjs, worldObj{uint32(i), tIdx, x, y, z})
		}
	}

	var spawns []PowerItemSpawn
	for i := int32(0); i < itemCount; i++ {
		itemAddr := firstItemAddr + uint32(i)*ScenarioItemStride
		tagIdxRaw, _ := mem.ReadS32(itemAddr + OffScenItemTagIndex)
		if tagIdxRaw < 0 {
			continue
		}
		tagIdx := int16(tagIdxRaw)
		tagName, err := r.readTagName(tagIdx)
		if err != nil || tagName == "" {
			continue
		}
		interval := r.readSpawnInterval(tagIdx)
		if interval <= 0 {
			continue // not a power item
		}
		x, _ := mem.ReadF32(itemAddr + OffScenItemX)
		y, _ := mem.ReadF32(itemAddr + OffScenItemY)
		z, _ := mem.ReadF32(itemAddr + OffScenItemZ)

		// Find the closest world object with this tag at game start.
		initialOID := uint32(0xFFFF)
		minDist := float32(math.MaxFloat32)
		for _, wo := range worldObjs {
			if wo.tagIdx != tagIdx {
				continue
			}
			dx, dy, dz := wo.x-x, wo.y-y, wo.z-z
			d := dx*dx + dy*dy + dz*dz
			if d < minDist {
				minDist = d
				initialOID = wo.objectID
			}
		}

		spawns = append(spawns, PowerItemSpawn{
			SpawnID:            len(spawns),
			Tag:                tagName,
			SpawnIntervalTicks: interval,
			X:                  x,
			Y:                  y,
			Z:                  z,
			InitialObjectID:    initialOID,
		})
	}
	return spawns, nil
}

func (r *Reader) readSpawnInterval(tagIdx int16) int16 {
	if r.tagInstBase < 0x80000000 {
		return 0
	}
	mem := r.inst.Mem
	tagInstEntry := r.tagInstBase + uint32(TagInstStride)*uint32(uint16(tagIdx))
	tagDataPtr, _ := mem.ReadU32(tagInstEntry + OffTagDataPtr)
	if tagDataPtr < 0x80000000 {
		return 0
	}
	// Read the pointer to the respawn interval table.
	intervalTablePtr, _ := mem.ReadU32(tagDataPtr + OffTagRespawnIntervalOff)
	if intervalTablePtr < 0x80000000 {
		return 0
	}
	interval, _ := mem.ReadS16(intervalTablePtr + OffTagRespawnInterval)
	return interval
}

// -------------------------------------------------------------------
// Tag name lookup
// -------------------------------------------------------------------

func (r *Reader) readTagName(tagIdx int16) (string, error) {
	if name, ok := r.tagNameCache[tagIdx]; ok {
		return name, nil
	}
	if r.tagInstBase < 0x80000000 {
		return "", nil
	}
	mem := r.inst.Mem
	tagInstEntry := r.tagInstBase + uint32(TagInstStride)*uint32(uint16(tagIdx))
	namePtr, err := mem.ReadU32(tagInstEntry + OffTagNamePtr)
	if err != nil || namePtr < 0x80000000 {
		return "", err
	}
	name := r.readHighString(namePtr)
	r.tagNameCache[tagIdx] = name
	return name, nil
}

func (r *Reader) readHighString(gva uint32) string {
	if gva < 0x80000000 {
		return ""
	}
	b, err := r.inst.Mem.ReadBytes(gva, 128)
	if err != nil {
		return ""
	}
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

// -------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------

func (r *Reader) ensureBases() error {
	if r.tagInstBase == 0 {
		base, err := r.inst.DerefLowPtr(AddrGlobalTagInstancesPtr)
		if err != nil {
			return err
		}
		r.tagInstBase = base
	}
	if r.ohdBase == 0 {
		base, err := r.inst.DerefLowPtr(AddrObjectHeaderDatumPtr)
		if err != nil {
			return err
		}
		r.ohdBase = base
	}
	return nil
}

// readLowString reads a null-terminated ASCII string from a cached low GVA host VA.
func (r *Reader) readLowString(lowGVA uint32, maxLen int) string {
	hva, err := r.inst.LowHVA(lowGVA)
	if err != nil {
		return ""
	}
	b, err := r.inst.Mem.ReadBytesAt(hva, maxLen)
	if err != nil {
		return ""
	}
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

// decodeUTF16LE decodes a null-terminated UTF-16LE byte slice into a Go string.
func decodeUTF16LE(b []byte) string {
	u16s := make([]uint16, len(b)/2)
	for i := range u16s {
		u16s[i] = binary.LittleEndian.Uint16(b[2*i:])
	}
	for i, c := range u16s {
		if c == 0 {
			u16s = u16s[:i]
			break
		}
	}
	return string(utf16.Decode(u16s))
}
