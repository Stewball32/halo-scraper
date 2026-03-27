package haloce

import (
	"xemu-cartographer/internal/scraper"
	"xemu-cartographer/internal/xemu"
)

// Xbox title ID for Halo: Combat Evolved.
const TitleID uint32 = 0x4D530004

// GametypeNames maps Halo CE gametype IDs to human-readable strings.
var GametypeNames = map[uint32]string{
	0:  "none",
	1:  "ctf",
	2:  "slayer",
	3:  "oddball",
	4:  "king",
	5:  "race",
	6:  "terminator",
	7:  "stub",
	12: "all",
	13: "all_except_ctf",
	14: "all_except_ctf_race",
}

// Game implements scraper.GameReader for Halo CE.
type Game struct {
	reader *Reader
}

// New creates a Halo CE GameReader for the given instance.
func New(inst *xemu.Instance, instanceName string) *Game {
	return &Game{reader: NewReader(inst, instanceName)}
}

func (g *Game) LowGVAs() []uint32 { return AllLowGVAs }

func (g *Game) ReadGameState() (scraper.GameState, uint32, error) {
	return g.reader.ReadGameState()
}

func (g *Game) ReadSnapshot() (scraper.SnapshotPayload, error) {
	return g.reader.ReadSnapshot()
}

func (g *Game) ReadTick(spawns []scraper.PowerItemSpawn, state *scraper.TickState) (scraper.TickResult, error) {
	return g.reader.ReadTick(spawns, state)
}

func (g *Game) DetectEvents(tick uint32, instance string, snap scraper.SnapshotPayload, result scraper.TickResult, state *scraper.TickState) []scraper.Envelope {
	return DetectEvents(tick, instance, snap, result, state)
}

func (g *Game) NewTickState() *scraper.TickState {
	return scraper.NewTickState()
}

func init() {
	scraper.Register(TitleID, func(inst *xemu.Instance, instanceName string) scraper.GameReader {
		return New(inst, instanceName)
	})
}
