package halo2

import (
	"encoding/binary"
	"strings"
	"unicode/utf16"

	"xemu-cartographer/internal/scraper"
	"xemu-cartographer/internal/xemu"
)

// Reader reads Halo 2 game state from a single xemu instance.
//
// Unlike Halo CE which uses low-GVA pointer dereferences to find datum arrays,
// Halo 2 data is accessed at fixed XBE-relative offsets. These are mapped into
// the high GVA space (>= 0x80000000) so the existing pread-based Mem reader
// can address them directly.
//
// Data sources:
//   - Session players (Section A): player names, teams, skills
//   - Variant info (Section A): map name, gametype
//   - Game stats (Section B): K/D/A per player (live, updated in real-time)
//   - Event buffer (Section A): kill/carry/score events with weapon info
type Reader struct {
	inst        *xemu.Instance
	name        string
	tickCounter uint32
	lastEventID uint32
}

// NewReader creates a Reader for the given instance.
func NewReader(inst *xemu.Instance, instanceName string) *Reader {
	return &Reader{
		inst: inst,
		name: instanceName,
	}
}

// -------------------------------------------------------------------
// Game state
// -------------------------------------------------------------------

// ReadGameState detects game state from session player presence and gametype.
func (r *Reader) ReadGameState() (scraper.GameState, uint32, error) {
	mem := r.inst.Mem

	gt, err := mem.ReadU8(GVAVariantInfo + VarOffGameType)
	if err != nil {
		return scraper.GameStateMenu, r.tickCounter, nil
	}

	// Check if first session player slot is populated.
	nameBytes, err := mem.ReadBytes(GVASessionPlayers+SessOffName, 2)
	if err != nil {
		return scraper.GameStateMenu, r.tickCounter, nil
	}
	hasPlayers := nameBytes[0] != 0 || nameBytes[1] != 0

	if hasPlayers && gt > 0 {
		r.tickCounter++
		return scraper.GameStateInGame, r.tickCounter, nil
	}

	if gt > 0 {
		return scraper.GameStatePreGame, r.tickCounter, nil
	}

	return scraper.GameStateMenu, r.tickCounter, nil
}

// -------------------------------------------------------------------
// Snapshot
// -------------------------------------------------------------------

func (r *Reader) ReadSnapshot() (scraper.SnapshotPayload, error) {
	mem := r.inst.Mem

	gt, _ := mem.ReadU8(GVAVariantInfo + VarOffGameType)
	gtName := GametypeNames[gt]
	if gtName == "" {
		gtName = "unknown"
	}

	mapName := r.readMapName()
	variantName := r.readVariantName()
	if variantName != "" {
		gtName = gtName + " (" + variantName + ")"
	}

	players := r.readSessionPlayers()
	snapPlayers := make([]scraper.SnapshotPlayer, len(players))
	teamCounts := map[uint8]int{}
	for i, p := range players {
		gs := r.readGameStats(i)
		snapPlayers[i] = scraper.SnapshotPlayer{
			Index:      i,
			Name:       p.name,
			Team:       uint32(p.team),
			Kills:      int16(gs.kills),
			Deaths:     int16(gs.deaths),
			Assists:    int16(gs.assists),
			TeamKills:  int16(gs.betrayals),
			Suicides:   int16(gs.suicides),
			KillStreak: gs.bestSpree,
			ShotsFired: int32(gs.shotsFired),
			ShotsHit:   int16(gs.shotsHit),
		}
		teamCounts[p.team]++
	}

	isTeamGame := false
	for _, count := range teamCounts {
		if count >= 2 {
			isTeamGame = true
			break
		}
	}

	var teamScores []scraper.TeamScore
	if isTeamGame {
		teamSums := make(map[uint8]int32)
		for i, p := range players {
			gs := r.readGameStats(i)
			teamSums[p.team] += int32(gs.kills)
		}
		for tid, score := range teamSums {
			teamScores = append(teamScores, scraper.TeamScore{
				Team:  uint32(tid),
				Score: score,
			})
		}
	}

	return scraper.SnapshotPayload{
		Map:        mapName,
		Gametype:   gtName,
		IsTeamGame: isTeamGame,
		TeamScores: teamScores,
		Players:    snapPlayers,
	}, nil
}

// -------------------------------------------------------------------
// Tick
// -------------------------------------------------------------------

func (r *Reader) ReadTick(spawns []scraper.PowerItemSpawn, state *scraper.TickState) (scraper.TickResult, error) {
	players := r.readSessionPlayers()

	tickPlayers := make([]scraper.TickPlayer, 0, len(players))
	internalPlayers := make([]scraper.InternalPlayerState, 0, len(players))

	for i, p := range players {
		gs := r.readGameStats(i)

		tp := scraper.TickPlayer{
			Index:   i,
			Alive:   true,
			Health:  1.0,
			Shields: 1.0,
			Frags:   0,
			Plasmas: 0,
		}

		ip := scraper.InternalPlayerState{
			Index:        i,
			ParentObject: 0xFFFFFFFF, // on foot (unknown)
			Kills:        int16(gs.kills),
			Deaths:       int16(gs.deaths),
			Assists:      int16(gs.assists),
			TeamKills:    int16(gs.betrayals),
			Suicides:     int16(gs.suicides),
			KillStreak:   gs.bestSpree,
			ShotsFired:   int32(gs.shotsFired),
			ShotsHit:     int16(gs.shotsHit),
		}
		_ = p // name/team already used in snapshot; tick only needs stats

		tickPlayers = append(tickPlayers, tp)
		internalPlayers = append(internalPlayers, ip)
	}

	return scraper.TickResult{
		Payload: scraper.TickPayload{
			Players: tickPlayers,
		},
		InternalPlayers: internalPlayers,
	}, nil
}

// -------------------------------------------------------------------
// Session players
// -------------------------------------------------------------------

type sessionPlayer struct {
	name  string
	team  uint8
	skill uint8
}

func (r *Reader) readSessionPlayers() []sessionPlayer {
	mem := r.inst.Mem
	var players []sessionPlayer

	for i := 0; i < MaxPlayers; i++ {
		base := GVASessionPlayers + uint32(i)*SessionPlayerStride
		nameBytes, err := mem.ReadBytes(base+SessOffName, 32)
		if err != nil {
			continue
		}
		name := decodeUTF16LE(nameBytes)
		if name == "" || !isPrintableASCII(name) {
			continue
		}
		team, _ := mem.ReadU8(base + SessOffTeamIndex)
		skill, _ := mem.ReadU8(base + SessOffDisplayedSkill)
		players = append(players, sessionPlayer{name: name, team: team, skill: skill})
	}
	return players
}

// -------------------------------------------------------------------
// Game stats
// -------------------------------------------------------------------

type gameStats struct {
	kills, assists, deaths, betrayals, suicides, bestSpree uint16
	shotsFired, shotsHit, headshots                        uint16
}

func (r *Reader) readGameStats(playerIndex int) gameStats {
	mem := r.inst.Mem

	var base uint32
	if playerIndex < 5 {
		base = GVAGameStats + uint32(playerIndex)*StatsStride
	} else {
		base = GVAGameStatsExtra + uint32(playerIndex-5)*StatsStride
	}

	data, err := mem.ReadBytes(base, 0x0E)
	if err != nil || len(data) < 0x0E {
		return gameStats{}
	}

	gs := gameStats{
		kills:     binary.LittleEndian.Uint16(data[GSOffKills:]),
		assists:   binary.LittleEndian.Uint16(data[GSOffAssists:]),
		deaths:    binary.LittleEndian.Uint16(data[GSOffDeaths:]),
		betrayals: binary.LittleEndian.Uint16(data[GSOffBetrayals:]),
		suicides:  binary.LittleEndian.Uint16(data[GSOffSuicides:]),
		bestSpree: binary.LittleEndian.Uint16(data[GSOffBestSpree:]),
	}

	// Sum weapon stats across all 41 weapons for shots/hits/headshots.
	weaponBase := base + OffWeaponStatsInStride
	weaponData, err := mem.ReadBytes(weaponBase, int(WeaponStatSize*WeaponCount))
	if err == nil && len(weaponData) >= int(WeaponStatSize*WeaponCount) {
		for w := 0; w < WeaponCount; w++ {
			off := w * int(WeaponStatSize)
			gs.shotsFired += binary.LittleEndian.Uint16(weaponData[off+int(WepOffShotsFired):])
			gs.shotsHit += binary.LittleEndian.Uint16(weaponData[off+int(WepOffShotsHit):])
			gs.headshots += binary.LittleEndian.Uint16(weaponData[off+int(WepOffHeadshots):])
		}
	}

	return gs
}

// -------------------------------------------------------------------
// Map and variant names
// -------------------------------------------------------------------

func (r *Reader) readMapName() string {
	mem := r.inst.Mem
	scenBytes, err := mem.ReadBytes(GVAVariantInfo+VarOffScenario, 256)
	if err != nil {
		return ""
	}
	scenario := ""
	for i, b := range scenBytes {
		if b == 0 {
			scenario = string(scenBytes[:i])
			break
		}
	}
	if scenario == "" {
		return ""
	}
	// Scenario path uses backslashes; last component is the map name.
	if idx := strings.LastIndex(scenario, "\\"); idx >= 0 {
		scenario = scenario[idx+1:]
	}
	if display, ok := mapDisplayNames[scenario]; ok {
		return display
	}
	return scenario
}

func (r *Reader) readVariantName() string {
	mem := r.inst.Mem
	nameBytes, err := mem.ReadBytes(GVAVariantInfo+VarOffName, 32)
	if err != nil {
		return ""
	}
	name := decodeUTF16LE(nameBytes)
	if !isPrintableASCII(name) {
		return ""
	}
	return name
}

// -------------------------------------------------------------------
// Event buffer
// -------------------------------------------------------------------

// readNewEvents reads new kill/carry/score events from the circular event buffer.
func (r *Reader) readNewEvents() []rawEvent {
	mem := r.inst.Mem

	countRaw, err := mem.ReadU32(GVAEventCount)
	if err != nil || countRaw == 0 {
		return nil
	}

	var newCount uint32
	if countRaw >= r.lastEventID {
		newCount = countRaw - r.lastEventID
	} else {
		newCount = countRaw + (0xFFFFFFFF - r.lastEventID) + 1
	}
	if newCount == 0 {
		return nil
	}
	if newCount > EventBufferCapacity {
		newCount = EventBufferCapacity
		r.lastEventID = countRaw - EventBufferCapacity
	}

	var events []rawEvent
	for i := uint32(0); i < newCount; i++ {
		eventIndex := r.lastEventID + i
		bufferIndex := eventIndex % EventBufferCapacity
		offset := GVAEventBuffer + bufferIndex*EventStructSize

		data, err := mem.ReadBytes(offset, int(EventStructSize))
		if err != nil || len(data) < int(EventStructSize) {
			continue
		}

		evtType := data[EvtOffType]
		if evtType == 0 || evtType > 3 {
			continue
		}

		events = append(events, rawEvent{
			evtType:   evtType,
			source:    data[EvtOffSource],
			effected:  data[EvtOffEffected],
			weapon:    binary.LittleEndian.Uint32(data[EvtOffWeapon:]),
			carryType: binary.LittleEndian.Uint32(data[EvtOffCarryType:]),
			timestamp: int32(binary.LittleEndian.Uint32(data[EvtOffTimestamp:])),
		})
	}

	r.lastEventID = countRaw
	return events
}

type rawEvent struct {
	evtType   uint8
	source    uint8
	effected  uint8
	weapon    uint32
	carryType uint32
	timestamp int32
}

// -------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------

func decodeUTF16LE(b []byte) string {
	if len(b) < 2 {
		return ""
	}
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

func isPrintableASCII(s string) bool {
	for _, c := range s {
		if c < 0x20 || c > 0x7E {
			return false
		}
	}
	return len(s) > 0
}
