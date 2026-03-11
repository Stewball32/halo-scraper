// scraper initialises all xemu instances and polls each one for Halo CE game
// state. Run as root (needs /proc/<pid>/mem read access):
//
//	sudo go run ./cmd/scraper
package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"

	"halo-scraper/internal/halo"
	"halo-scraper/internal/pb"
	"halo-scraper/internal/ws"
	"halo-scraper/internal/xemu"
)

type config struct {
	Server struct {
		Addr   string `toml:"addr"`
		TickHz int    `toml:"tick_hz"`
	} `toml:"server"`
	PocketBase struct {
		Enabled bool   `toml:"enabled"`
		URL     string `toml:"url"`
	} `toml:"pocketbase"`
	Performance struct {
		IdleMs int  `toml:"idle_ms"`
		GameMs int  `toml:"game_ms"`
		Events bool `toml:"events"`
	} `toml:"performance"`
	Hosts []hostCfg `toml:"hosts"`
}

type hostCfg struct {
	Name    string `toml:"name"`
	QMPSock string `toml:"qmp_sock"`
	IdleMs  int    `toml:"idle_ms"` // 0 = use global default
	GameMs  int    `toml:"game_ms"` // 0 = use global default
	Events  *bool  `toml:"events"`  // nil = use global default
}

type pollSettings struct {
	idle         time.Duration
	game         time.Duration
	events       bool
	tickInterval time.Duration
}

func (h hostCfg) resolve(cfg config) pollSettings {
	idle := time.Duration(cfg.Performance.IdleMs) * time.Millisecond
	if h.IdleMs > 0 {
		idle = time.Duration(h.IdleMs) * time.Millisecond
	}
	game := time.Duration(cfg.Performance.GameMs) * time.Millisecond
	if h.GameMs > 0 {
		game = time.Duration(h.GameMs) * time.Millisecond
	}
	events := cfg.Performance.Events
	if h.Events != nil {
		events = *h.Events
	}
	tickHz := cfg.Server.TickHz
	if tickHz <= 0 {
		tickHz = 30
	}
	return pollSettings{
		idle:         idle,
		game:         game,
		events:       events,
		tickInterval: time.Second / time.Duration(tickHz),
	}
}

func loadConfig() config {
	const filename = "halo-scraper.toml"

	// Pre-populate defaults — TOML decode only overwrites keys present in the file,
	// so omitted keys keep these values. This also fixes the bool zero-value footgun
	// (enabled/events would silently default to false without this).
	cfg := config{}
	cfg.Server.Addr = ":9000"
	cfg.Server.TickHz = 30
	cfg.PocketBase.Enabled = true
	cfg.PocketBase.URL = "http://localhost:8090"
	cfg.Performance.IdleMs = 500
	cfg.Performance.GameMs = 10
	cfg.Performance.Events = true

	// Look next to the binary first, then fall back to working directory.
	candidates := []string{}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), filename))
	}
	candidates = append(candidates, filename)

	for _, path := range candidates {
		if _, err := toml.DecodeFile(path, &cfg); err == nil {
			log.Printf("config: loaded %s", path)
			return cfg
		}
	}

	log.Fatalf("config: could not find %s next to binary or in working directory", filename)
	return cfg
}

// hostStatus tracks the connection state of one xemu instance.
type hostStatus struct {
	State string    // "connecting" | "online" | "offline"
	Error string    // last error message; empty when online
	Since time.Time // when State last changed
}

func setStatus(mu *sync.Mutex, s *hostStatus, state, errMsg string) {
	mu.Lock()
	s.State = state
	s.Error = errMsg
	s.Since = time.Now()
	mu.Unlock()
}

// ringLog is an io.Writer that keeps the last maxLines log lines in memory.
type ringLog struct {
	mu    sync.Mutex
	lines []string
}

func (r *ringLog) Write(p []byte) (int, error) {
	line := strings.TrimRight(string(p), "\n")
	r.mu.Lock()
	r.lines = append(r.lines, line)
	if len(r.lines) > 500 {
		r.lines = r.lines[len(r.lines)-500:]
	}
	r.mu.Unlock()
	return len(p), nil
}

func (r *ringLog) Lines() []string {
	r.mu.Lock()
	out := make([]string, len(r.lines))
	copy(out, r.lines)
	r.mu.Unlock()
	return out
}

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)

	rl := &ringLog{}
	log.SetOutput(io.MultiWriter(os.Stderr, rl))

	cfg := loadConfig()

	if len(cfg.Hosts) == 0 {
		log.Fatal("config: no hosts defined")
	}

	hub := ws.NewHub()

	// Per-host status and reconnect channels.
	var statusMu sync.Mutex
	statuses := make(map[string]*hostStatus, len(cfg.Hosts))
	reconnectChans := make(map[string]chan struct{}, len(cfg.Hosts))
	for _, host := range cfg.Hosts {
		statuses[host.Name] = &hostStatus{State: "connecting", Since: time.Now()}
		reconnectChans[host.Name] = make(chan struct{}, 1)
	}

	startTime := time.Now()

	// GET /api/status
	hub.Handle("/api/status", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		type hostJSON struct {
			State string `json:"state"`
			Since string `json:"since"`
			Error string `json:"error"`
		}
		statusMu.Lock()
		hosts := make(map[string]hostJSON, len(statuses))
		for name, s := range statuses {
			hosts[name] = hostJSON{
				State: s.State,
				Since: s.Since.UTC().Format(time.RFC3339),
				Error: s.Error,
			}
		}
		statusMu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"uptime_s":   int(time.Since(startTime).Seconds()),
			"ws_clients": hub.ClientCount(),
			"hosts":      hosts,
		})
	}))

	// GET /api/logs
	hub.Handle("/api/logs", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_ = json.NewEncoder(w).Encode(rl.Lines())
	}))

	// POST /api/hosts/{name}/reconnect
	hub.Handle("/api/hosts/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// Path: /api/hosts/{name}/reconnect
		path := strings.TrimPrefix(r.URL.Path, "/api/hosts/")
		name := strings.TrimSuffix(path, "/reconnect")
		ch, ok := reconnectChans[name]
		if !ok {
			http.Error(w, "unknown host", http.StatusNotFound)
			return
		}
		select {
		case ch <- struct{}{}:
		default: // already queued
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusNoContent)
	}))

	go func() {
		if err := hub.ListenAndServe(cfg.Server.Addr); err != nil {
			log.Fatalf("ws: %v", err)
		}
	}()

	var pbClient *pb.Client
	if cfg.PocketBase.Enabled {
		pbClient = pb.NewClient(cfg.PocketBase.URL)
	}

	for _, host := range cfg.Hosts {
		host := host
		go runHost(host, cfg, hub, pbClient, statuses[host.Name], &statusMu, reconnectChans[host.Name])
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("shutting down")
}

// connect tries to initialise inst every 2 seconds for up to 10 seconds.
// Returns the ready instance on success, or an error if the window expires.
func connect(host hostCfg) (*xemu.Instance, error) {
	deadline := time.Now().Add(10 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		inst := &xemu.Instance{Name: host.Name, QMPSock: host.QMPSock}
		if err := inst.Init(halo.AllLowGVAs); err == nil {
			return inst, nil
		} else {
			lastErr = err
		}
		time.Sleep(2 * time.Second)
	}
	return nil, lastErr
}

// runHost supervises one xemu instance: connects on startup, polls while alive,
// then waits for a manual reconnect signal before trying again.
func runHost(
	host hostCfg,
	cfg config,
	hub *ws.Hub,
	pbClient *pb.Client,
	status *hostStatus,
	mu *sync.Mutex,
	reconnect <-chan struct{},
) {
	settings := host.resolve(cfg)

	tryConnect := func() {
		setStatus(mu, status, "connecting", "")
		log.Printf("%s: initialising...", host.Name)
		inst, err := connect(host)
		if err != nil {
			log.Printf("%s: init failed: %v", host.Name, err)
			setStatus(mu, status, "offline", err.Error())
			return
		}
		log.Printf("%s: ready", host.Name)
		hub.RegisterInstance(host.Name)
		setStatus(mu, status, "online", "")

		reader := halo.NewReader(inst, host.Name)
		state := halo.NewTickState()
		poll(reader, host.Name, hub, pbClient, state, settings)

		// poll returned — instance is dead.
		hub.UnregisterInstance(host.Name)
		inst.Close()
		log.Printf("%s: lost connection", host.Name)
		setStatus(mu, status, "offline", "lost connection")
	}

	tryConnect()
	for range reconnect {
		tryConnect()
	}
}

func poll(
	reader *halo.Reader,
	name string,
	hub *ws.Hub,
	pbClient *pb.Client,
	state *halo.TickState,
	settings pollSettings,
) {
	var (
		prevTick          uint32
		prevState         halo.GameState
		cachedSpawns      []halo.PowerItemSpawn
		spawnsLoaded      bool
		lastTickBroadcast time.Time
		errCount          int
	)

	const maxErrs = 5

	for {
		gameState, tick, err := reader.ReadGameState()
		if err != nil {
			errCount++
			log.Printf("%s: read error (%d/%d): %v", name, errCount, maxErrs, err)
			if errCount >= maxErrs {
				log.Printf("%s: %d consecutive read errors, giving up", name, maxErrs)
				return
			}
			time.Sleep(time.Second)
			continue
		}
		errCount = 0

		// Slow poll when not actively in game.
		if gameState != halo.GameStateInGame {
			if gameState != prevState {
				handleStateTransition(name, tick, gameState, prevState, reader, hub, pbClient, state)
				prevState = gameState
				spawnsLoaded = false
				cachedSpawns = nil
			}
			time.Sleep(settings.idle)
			continue
		}

		// In game: wait for tick to advance.
		if tick == prevTick {
			time.Sleep(settings.game)
			continue
		}
		prevTick = tick

		// First tick in game state: send snapshot + game_start event.
		if prevState != halo.GameStateInGame {
			snap, err := reader.ReadSnapshot()
			if err != nil {
				log.Printf("%s: snapshot error: %v", name, err)
			} else {
				snap.GameState = halo.GameStateInGame
				cachedSpawns = snap.PowerItemSpawns
				spawnsLoaded = true

				// Initialise power item trackers.
				state.InitPowerItems(cachedSpawns)

				// Broadcast snapshot.
				broadcastSnapshot(name, tick, snap, hub, pbClient)

				// Emit game_start event.
				startEvt := halo.MakeEnvelope("event", name, tick, map[string]any{
					"event_type":  halo.EventGameStart,
					"map":         snap.Map,
					"gametype":    snap.Gametype,
					"score_limit": snap.ScoreLimit,
				})
				broadcastEnvelope(startEvt, hub, pbClient)
			}
			prevState = halo.GameStateInGame
		}

		if !spawnsLoaded {
			snap, err := reader.ReadSnapshot()
			if err == nil {
				cachedSpawns = snap.PowerItemSpawns
				spawnsLoaded = true
				state.InitPowerItems(cachedSpawns)
			}
		}

		// Read full tick.
		tickResult, err := reader.ReadTick(cachedSpawns, state)
		if err != nil {
			log.Printf("%s: tick error: %v", name, err)
			continue
		}

		// Broadcast tick message, throttled to tick_hz.
		if time.Since(lastTickBroadcast) >= settings.tickInterval {
			tickEnv := halo.MakeEnvelope("tick", name, tick, tickResult.Payload)
			if msg, err := json.Marshal(tickEnv); err == nil {
				hub.Broadcast(name, msg)
			}
			lastTickBroadcast = time.Now()
		}

		// Detect and broadcast events (skipped if events = false).
		if settings.events {
			snap, _ := reader.ReadSnapshot()
			snap.GameState = halo.GameStateInGame
			snap.PowerItemSpawns = cachedSpawns

			events := halo.DetectEvents(tick, name, snap, tickResult, state)
			for _, evt := range events {
				broadcastEnvelope(evt, hub, pbClient)
			}
		}

		// Update state.PrevTick.
		state.PrevTick = tick
	}
}

func handleStateTransition(
	name string,
	tick uint32,
	newState halo.GameState,
	prevState halo.GameState,
	reader *halo.Reader,
	hub *ws.Hub,
	pbClient *pb.Client,
	state *halo.TickState,
) {
	snap, err := reader.ReadSnapshot()
	if err != nil {
		log.Printf("%s: snapshot error on transition: %v", name, err)
		return
	}
	snap.GameState = newState

	log.Printf("%s: state → %s (tick %d)", name, newState, tick)

	// game_end: post-game transition from in-game.
	if newState == halo.GameStatePostGame && prevState == halo.GameStateInGame {
		teamScores := snap.TeamScores
		scores := make([]map[string]any, 0, len(snap.Players))
		for _, p := range snap.Players {
			scores = append(scores, map[string]any{
				"index":       p.Index,
				"name":        p.Name,
				"team":        p.Team,
				"kills":       p.Kills,
				"deaths":      p.Deaths,
				"assists":     p.Assists,
				"team_kills":  p.TeamKills,
				"suicides":    p.Suicides,
				"shots_fired": p.ShotsFired,
				"shots_hit":   p.ShotsHit,
			})
		}
		endPayload := map[string]any{
			"event_type":  halo.EventGameEnd,
			"team_scores": teamScores,
			"scores":      scores,
		}
		endEvt := halo.MakeEnvelope("event", name, tick, endPayload)
		broadcastEnvelope(endEvt, hub, pbClient)

		// Reset inter-tick state for next game.
		*state = *halo.NewTickState()
	}

	broadcastSnapshot(name, tick, snap, hub, pbClient)
}

func broadcastSnapshot(name string, tick uint32, snap halo.SnapshotPayload, hub *ws.Hub, pbClient *pb.Client) {
	env := halo.MakeEnvelope("snapshot", name, tick, snap)
	msg, err := json.Marshal(env)
	if err != nil {
		log.Printf("%s: marshal snapshot: %v", name, err)
		return
	}
	hub.BroadcastSnapshot(name, msg)
	if pbClient != nil {
		pbClient.PostSnapshot(name, tick, snap)
	}
}

func broadcastEnvelope(env halo.Envelope, hub *ws.Hub, pbClient *pb.Client) {
	msg, err := json.Marshal(env)
	if err != nil {
		return
	}
	hub.Broadcast(env.Instance, msg)

	if pbClient == nil {
		return
	}

	// Extract event_type from payload for PocketBase.
	var p map[string]any
	if json.Unmarshal(env.Payload, &p) == nil {
		if et, ok := p["event_type"].(string); ok {
			pbClient.PostEvent(env.Instance, env.Tick, et, p)
		}
	}
}
