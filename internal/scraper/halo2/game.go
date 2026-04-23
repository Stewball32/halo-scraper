package halo2

import (
	"xemu-cartographer/internal/scraper"
	"xemu-cartographer/internal/xemu"
)

// Xbox title ID for Halo 2.
const TitleID uint32 = 0x4D530064

// GametypeNames maps Halo 2 gametype IDs to human-readable strings.
// Only slayer (2) has been verified in xemu; others are from HaloCaster.
var GametypeNames = map[uint8]string{
	0: "none",
	1: "ctf",
	2: "slayer",
	3: "oddball",
	4: "koth",
	7: "juggernaut",
	8: "territories",
	9: "assault",
}

// DamageReportingNames maps the Halo 2 DamageReportingType enum to weapon/damage names.
// UNVERIFIED — from HaloCaster. Event buffer is non-functional so these are untested.
var DamageReportingNames = map[uint8]string{
	0:  "guardians",
	1:  "fall damage",
	2:  "collision",
	3:  "melee",
	4:  "explosion",
	5:  "magnum",
	6:  "plasma pistol",
	7:  "needler",
	8:  "smg",
	9:  "plasma rifle",
	10: "battle rifle",
	11: "carbine",
	12: "shotgun",
	13: "sniper rifle",
	14: "beam rifle",
	15: "rocket launcher",
	16: "fuel rod",
	17: "brute shot",
	18: "disintegrator",
	19: "brute plasma rifle",
	20: "energy sword",
	21: "frag grenade",
	22: "plasma grenade",
	23: "flag melee",
	24: "bomb melee",
	25: "bomb explosion",
	26: "oddball melee",
	27: "turret",
	28: "turret plasma",
	29: "banshee",
	30: "ghost",
	31: "mongoose",
	32: "scorpion",
	33: "spectre",
	34: "warthog",
	35: "wraith",
	36: "tank",
	37: "sentinel beam",
	38: "sentinel rpg",
	39: "teleporter",
}

// mapDisplayNames converts internal scenario names to display names.
// Only "lockout" has been verified in xemu; others are from community sources.
var mapDisplayNames = map[string]string{
	"beavercreek":    "Beaver Creek",
	"burial_mounds":  "Burial Mounds",
	"coagulation":    "Coagulation",
	"colossus":       "Colossus",
	"cyclotron":      "Ivory Tower",
	"foundation":     "Foundation",
	"headlong":       "Headlong",
	"lockout":        "Lockout",
	"midship":        "Midship",
	"waterworks":     "Waterworks",
	"zanzibar":       "Zanzibar",
	"ascension":      "Ascension",
	"deltatap":       "Sanctuary",
	"dune":           "Relic",
	"elongation":     "Elongation",
	"gemini":         "Gemini",
	"triplicate":     "Terminal",
	"turf":           "Turf",
	"containment":    "Containment",
	"warlock":        "Warlock",
	"street_sweeper": "District",
	"needle":         "Uplift",
	"backwash":       "Backwash",
}

// Game implements scraper.GameReader for Halo 2.
type Game struct {
	reader *Reader
}

// New creates a Halo 2 GameReader for the given instance.
func New(inst *xemu.Instance, instanceName string) *Game {
	return &Game{reader: NewReader(inst, instanceName)}
}

// LowGVAs returns an empty slice — Halo 2 accesses all data via constructed
// high GVAs (XBE-relative offsets mapped into the high GVA space), so no
// low-GVA translation is needed beyond the detection GVAs.
func (g *Game) LowGVAs() []uint32 { return nil }

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
	return g.reader.DetectEvents(tick, instance, snap, result, state)
}

func (g *Game) NewTickState() *scraper.TickState {
	return scraper.NewTickState()
}

// XboxName returns "" — no known offset for the xbox console name in Halo 2 yet.
func (g *Game) XboxName() string { return "" }

func init() {
	scraper.Register(TitleID, func(inst *xemu.Instance, instanceName string) scraper.GameReader {
		return New(inst, instanceName)
	})
}
