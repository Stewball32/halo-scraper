// scraper initialises all xemu instances, auto-detects the running game, and
// polls each one for game state. Run as root (needs /proc/<pid>/mem read access):
//
//	sudo go run ./cmd/cartographer
package main

import (
	"context"
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

	"xemu-cartographer/internal/discovery"
	"xemu-cartographer/internal/pb"
	"xemu-cartographer/internal/podman"
	"xemu-cartographer/internal/scraper"
	"xemu-cartographer/internal/ws"
	"xemu-cartographer/internal/xemu"

	// Register game scrapers.
	_ "xemu-cartographer/internal/scraper/halo2"
	_ "xemu-cartographer/internal/scraper/haloce"
)

// ---------------------------------------------------------------------------
// Config types
// ---------------------------------------------------------------------------

type config struct {
	Server struct {
		Addr   string `toml:"addr"`
		TickHz int    `toml:"tick_hz"`
	} `toml:"server"`
	SocketDir    string `toml:"socket_dir"`
	SocketPollMs int    `toml:"socket_poll_ms"`
	PocketBase   struct {
		Enabled bool   `toml:"enabled"`
		URL     string `toml:"url"`
	} `toml:"pocketbase"`
	Performance struct {
		IdleMs int  `toml:"idle_ms"`
		GameMs int  `toml:"game_ms"`
		Events bool `toml:"events"`
	} `toml:"performance"`
	Containers podman.Config `toml:"containers"`
	Hosts      []hostCfg     `toml:"hosts"`
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
	const filename = "xemu-cartographer.toml"

	// Pre-populate defaults — TOML decode only overwrites keys present in the file,
	// so omitted keys keep these values. This also fixes the bool zero-value footgun
	// (enabled/events would silently default to false without this).
	cfg := config{}
	cfg.Server.Addr = ":9000"
	cfg.Server.TickHz = 30
	cfg.SocketPollMs = 2000
	cfg.PocketBase.Enabled = true
	cfg.PocketBase.URL = "http://localhost:8090"
	cfg.Performance.IdleMs = 500
	cfg.Performance.GameMs = 10
	cfg.Performance.Events = true

	// Container defaults.
	cfg.Containers.PortBase = 3100
	cfg.Containers.PortStride = 10
	cfg.Containers.StateFile = "./containers/state.json"
	cfg.Containers.ShmSize = "1g"
	cfg.Containers.BrowserShmSize = "2gb"

	// Look next to the binary first, then fall back to working directory.
	candidates := []string{}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), filename))
	}
	candidates = append(candidates, filename)

	for _, path := range candidates {
		if _, err := toml.DecodeFile(path, &cfg); err == nil {
			log.Printf("config: loaded %s", path)
			resolveConfigPaths(path, &cfg)
			return cfg
		}
	}

	log.Fatalf("config: could not find %s next to binary or in working directory", filename)
	return cfg
}

// resolveConfigPaths makes all relative paths in the config absolute, anchored
// to the directory containing the config file. This ensures volume mounts and
// file references work regardless of the working directory.
func resolveConfigPaths(cfgFile string, cfg *config) {
	base, err := filepath.Abs(filepath.Dir(cfgFile))
	if err != nil {
		return
	}
	resolve := func(p *string) {
		if *p != "" && !filepath.IsAbs(*p) {
			*p = filepath.Join(base, *p)
		}
	}
	resolve(&cfg.SocketDir)
	resolve(&cfg.Containers.SocketDir)
	resolve(&cfg.Containers.SharedDir)
	resolve(&cfg.Containers.InitDir)
	resolve(&cfg.Containers.ConfigsDir)
	resolve(&cfg.Containers.BrowserDir)
	resolve(&cfg.Containers.BrowserInitDir)
	resolve(&cfg.Containers.StateFile)
}

// ---------------------------------------------------------------------------
// Host status tracking
// ---------------------------------------------------------------------------

// hostStatus tracks the connection state of one xemu instance.
type hostStatus struct {
	State    string    // "connecting" | "online" | "offline"
	Error    string    // last error message; empty when online
	Since    time.Time // when State last changed
	XboxName string    // console name of the xbox running the game ("" if unknown)
}

// hostTracker manages per-host status and reconnect channels. It is safe for
// concurrent use.
type hostTracker struct {
	mu             sync.Mutex
	statuses       map[string]*hostStatus
	reconnectChans map[string]chan struct{}
	cancelFuncs    map[string]context.CancelFunc
}

func newHostTracker() *hostTracker {
	return &hostTracker{
		statuses:       make(map[string]*hostStatus),
		reconnectChans: make(map[string]chan struct{}),
		cancelFuncs:    make(map[string]context.CancelFunc),
	}
}

func (t *hostTracker) register(name string, cancel context.CancelFunc) (<-chan struct{}, *hostStatus) {
	t.mu.Lock()
	defer t.mu.Unlock()
	s := &hostStatus{State: "connecting", Since: time.Now()}
	t.statuses[name] = s
	ch := make(chan struct{}, 1)
	t.reconnectChans[name] = ch
	t.cancelFuncs[name] = cancel
	return ch, s
}

func (t *hostTracker) unregister(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if cancel, ok := t.cancelFuncs[name]; ok {
		cancel()
	}
	delete(t.statuses, name)
	delete(t.reconnectChans, name)
	delete(t.cancelFuncs, name)
}

func (t *hostTracker) setStatus(name, state, errMsg string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if s, ok := t.statuses[name]; ok {
		s.State = state
		s.Error = errMsg
		s.Since = time.Now()
	}
}

func (t *hostTracker) setXboxName(name, xboxName string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if s, ok := t.statuses[name]; ok {
		s.XboxName = xboxName
	}
}

func (t *hostTracker) reconnect(name string) bool {
	t.mu.Lock()
	ch, ok := t.reconnectChans[name]
	t.mu.Unlock()
	if !ok {
		return false
	}
	select {
	case ch <- struct{}{}:
	default:
	}
	return true
}

func (t *hostTracker) cancel(name string) {
	t.mu.Lock()
	cancel, ok := t.cancelFuncs[name]
	t.mu.Unlock()
	if ok {
		cancel()
	}
}

func (t *hostTracker) snapshot() map[string]hostStatus {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make(map[string]hostStatus, len(t.statuses))
	for k, v := range t.statuses {
		out[k] = *v
	}
	return out
}

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)

	rl := &ringLog{}
	log.SetOutput(io.MultiWriter(os.Stderr, rl))

	cfg := loadConfig()

	useSocketDir := cfg.SocketDir != ""
	if !useSocketDir && len(cfg.Hosts) == 0 {
		log.Fatal("config: no hosts defined and socket_dir is empty")
	}

	hub := ws.NewHub()
	tracker := newHostTracker()
	startTime := time.Now()

	// Build host override map from [[hosts]] entries.
	hostOverrides := make(map[string]hostCfg, len(cfg.Hosts))
	for _, h := range cfg.Hosts {
		hostOverrides[h.Name] = h
	}

	// --- HTTP endpoints ---

	// GET /api/status
	hub.Handle("/api/status", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		type hostJSON struct {
			State    string `json:"state"`
			Since    string `json:"since"`
			Error    string `json:"error"`
			XboxName string `json:"xbox_name"`
		}
		snap := tracker.snapshot()
		hosts := make(map[string]hostJSON, len(snap))
		for name, s := range snap {
			hosts[name] = hostJSON{
				State:    s.State,
				Since:    s.Since.UTC().Format(time.RFC3339),
				Error:    s.Error,
				XboxName: s.XboxName,
			}
		}

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
		path := strings.TrimPrefix(r.URL.Path, "/api/hosts/")
		name := strings.TrimSuffix(path, "/reconnect")
		if !tracker.reconnect(name) {
			http.Error(w, "unknown host", http.StatusNotFound)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusNoContent)
	}))

	// --- Podman container API ---

	var mgr *podman.Manager
	if cfg.Containers.Enabled {
		// Inherit socket_dir so containers place sockets where the watcher looks.
		containerCfg := cfg.Containers
		if containerCfg.SocketDir == "" {
			containerCfg.SocketDir = cfg.SocketDir
		}
		var err error
		mgr, err = podman.NewManager(containerCfg)
		if err != nil {
			log.Fatalf("podman: %v", err)
		}
		registerContainerAPI(hub, mgr)
	}

	go func() {
		if err := hub.ListenAndServe(cfg.Server.Addr); err != nil {
			log.Fatalf("ws: %v", err)
		}
	}()

	var pbClient *pb.Client
	if cfg.PocketBase.Enabled {
		pbClient = pb.NewClient(cfg.PocketBase.URL)
	}

	// Top-level context cancelled on SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if useSocketDir {
		// --- Socket directory discovery mode ---
		pollInterval := time.Duration(cfg.SocketPollMs) * time.Millisecond
		if pollInterval <= 0 {
			pollInterval = 2 * time.Second
		}

		watcher := discovery.NewWatcher(cfg.SocketDir, pollInterval,
			// onAdd: new connectable socket found.
			func(name, sockPath string) {
				host := hostCfg{Name: name, QMPSock: sockPath}
				if override, ok := hostOverrides[name]; ok {
					host.IdleMs = override.IdleMs
					host.GameMs = override.GameMs
					host.Events = override.Events
				}

				hostCtx, cancel := context.WithCancel(ctx)
				reconnect, status := tracker.register(name, cancel)
				_ = status

				go runHost(hostCtx, host, cfg, hub, pbClient, tracker, reconnect)
			},
			// onRemove: socket disappeared or went stale.
			func(name string) {
				log.Printf("%s: socket removed, cancelling", name)
				tracker.cancel(name)
				// Don't unregister yet — runHost will clean up when it exits.
			},
		)

		log.Printf("discovery: watching %s (poll every %s)", cfg.SocketDir, pollInterval)
		go watcher.Run(ctx)

		// Also start any static [[hosts]] that have explicit qmp_sock (hybrid mode).
		for _, host := range cfg.Hosts {
			if host.QMPSock == "" {
				continue // will be discovered via socket_dir
			}
			host := host
			hostCtx, cancel := context.WithCancel(ctx)
			reconnect, _ := tracker.register(host.Name, cancel)
			go runHost(hostCtx, host, cfg, hub, pbClient, tracker, reconnect)
		}
	} else {
		// --- Static [[hosts]] mode (backward compatible) ---
		for _, host := range cfg.Hosts {
			host := host
			hostCtx, cancel := context.WithCancel(ctx)
			reconnect, _ := tracker.register(host.Name, cancel)
			go runHost(hostCtx, host, cfg, hub, pbClient, tracker, reconnect)
		}
	}

	<-ctx.Done()
	log.Println("shutting down")
}

// ---------------------------------------------------------------------------
// Per-host lifecycle
// ---------------------------------------------------------------------------

// connect tries to initialise an xemu instance and auto-detect the running game.
// It performs a two-phase init: first translating the XBE header address for game
// detection, then re-initialising with the game-specific addresses.
func connect(ctx context.Context, host hostCfg) (*xemu.Instance, scraper.GameReader, error) {
	deadline := time.Now().Add(10 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		inst := &xemu.Instance{Name: host.Name, QMPSock: host.QMPSock}

		// Phase 1: translate detection GVAs and identify the game.
		if err := inst.Init(scraper.DetectionGVAs()); err != nil {
			lastErr = err
			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			case <-time.After(2 * time.Second):
			}
			continue
		}
		gameReader, titleID, err := scraper.Detect(inst, host.Name)
		inst.Close()
		if err != nil {
			lastErr = err
			log.Printf("%s: game detection failed (title ID 0x%08X): %v", host.Name, titleID, err)
			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			case <-time.After(2 * time.Second):
			}
			continue
		}
		log.Printf("%s: detected game (title ID 0x%08X)", host.Name, titleID)

		// Phase 2: re-init with the game-specific low GVAs.
		if err := inst.Init(gameReader.LowGVAs()); err != nil {
			lastErr = err
			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			case <-time.After(2 * time.Second):
			}
			continue
		}

		return inst, gameReader, nil
	}
	return nil, nil, lastErr
}

// runHost supervises one xemu instance: connects on startup, polls while alive,
// then waits for a reconnect signal or context cancellation before trying again.
func runHost(
	ctx context.Context,
	host hostCfg,
	cfg config,
	hub *ws.Hub,
	pbClient *pb.Client,
	tracker *hostTracker,
	reconnect <-chan struct{},
) {
	defer tracker.unregister(host.Name)

	settings := host.resolve(cfg)

	tryConnect := func() {
		tracker.setStatus(host.Name, "connecting", "")
		log.Printf("%s: initialising...", host.Name)
		inst, gameReader, err := connect(ctx, host)
		if err != nil {
			log.Printf("%s: init failed: %v", host.Name, err)
			tracker.setStatus(host.Name, "offline", err.Error())
			return
		}
		log.Printf("%s: ready", host.Name)
		hub.RegisterInstance(host.Name)
		tracker.setStatus(host.Name, "online", "")

		if xboxName := gameReader.XboxName(); xboxName != "" {
			log.Printf("%s: xbox name resolved to %q", host.Name, xboxName)
			tracker.setXboxName(host.Name, xboxName)
		}

		state := gameReader.NewTickState()
		poll(ctx, gameReader, host.Name, hub, pbClient, state, settings)

		// poll returned — instance is dead or context cancelled.
		hub.UnregisterInstance(host.Name)
		inst.Close()
		if ctx.Err() == nil {
			log.Printf("%s: lost connection", host.Name)
			tracker.setStatus(host.Name, "offline", "lost connection")
		}
	}

	const (
		retryMin = 10 * time.Second
		retryMax = 60 * time.Second
	)

	tryConnect()
	retryDelay := retryMin

	for {
		// Only arm the auto-retry timer when offline.
		// A nil channel blocks forever in select, which is what we want when online.
		var retryTimer <-chan time.Time
		if s, ok := tracker.snapshot()[host.Name]; ok && s.State == "offline" {
			retryTimer = time.After(retryDelay)
		}

		select {
		case <-ctx.Done():
			return
		case <-reconnect:
			retryDelay = retryMin
			tryConnect()
		case <-retryTimer:
			log.Printf("%s: auto-retrying connection (backoff %s)", host.Name, retryDelay)
			tryConnect()
			if s, ok := tracker.snapshot()[host.Name]; ok && s.State == "online" {
				retryDelay = retryMin
			} else if retryDelay < retryMax {
				retryDelay *= 2
			}
		}
	}
}

func poll(
	ctx context.Context,
	reader scraper.GameReader,
	name string,
	hub *ws.Hub,
	pbClient *pb.Client,
	state *scraper.TickState,
	settings pollSettings,
) {
	var (
		prevTick          uint32
		prevState         scraper.GameState
		cachedSpawns      []scraper.PowerItemSpawn
		spawnsLoaded      bool
		lastTickBroadcast time.Time
		errCount          int
	)

	const maxErrs = 5

	for {
		if ctx.Err() != nil {
			return
		}

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
		if gameState != scraper.GameStateInGame {
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
		if prevState != scraper.GameStateInGame {
			snap, err := reader.ReadSnapshot()
			if err != nil {
				log.Printf("%s: snapshot error: %v", name, err)
			} else {
				snap.GameState = scraper.GameStateInGame
				cachedSpawns = snap.PowerItemSpawns
				spawnsLoaded = true

				// Initialise power item trackers.
				state.InitPowerItems(cachedSpawns)

				// Broadcast snapshot.
				broadcastSnapshot(name, tick, snap, hub, pbClient)

				// Emit game_start event.
				startEvt := scraper.MakeEnvelope("event", name, tick, map[string]any{
					"event_type":  scraper.EventGameStart,
					"map":         snap.Map,
					"gametype":    snap.Gametype,
					"score_limit": snap.ScoreLimit,
				})
				broadcastEnvelope(startEvt, hub, pbClient)
			}
			prevState = scraper.GameStateInGame
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
			tickEnv := scraper.MakeEnvelope("tick", name, tick, tickResult.Payload)
			if msg, err := json.Marshal(tickEnv); err == nil {
				hub.Broadcast(name, msg)
			}
			lastTickBroadcast = time.Now()
		}

		// Detect and broadcast events (skipped if events = false).
		if settings.events {
			snap, _ := reader.ReadSnapshot()
			snap.GameState = scraper.GameStateInGame
			snap.PowerItemSpawns = cachedSpawns

			events := reader.DetectEvents(tick, name, snap, tickResult, state)
			for _, evt := range events {
				broadcastEnvelope(evt, hub, pbClient)
			}
		}

		// Update state.PrevTick.
		state.PrevTick = tick
	}
}

// ---------------------------------------------------------------------------
// Broadcast helpers (unchanged)
// ---------------------------------------------------------------------------

func handleStateTransition(
	name string,
	tick uint32,
	newState scraper.GameState,
	prevState scraper.GameState,
	reader scraper.GameReader,
	hub *ws.Hub,
	pbClient *pb.Client,
	state *scraper.TickState,
) {
	snap, err := reader.ReadSnapshot()
	if err != nil {
		log.Printf("%s: snapshot error on transition: %v", name, err)
		return
	}
	snap.GameState = newState

	log.Printf("%s: state → %s (tick %d)", name, newState, tick)

	// game_end: post-game transition from in-game.
	if newState == scraper.GameStatePostGame && prevState == scraper.GameStateInGame {
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
			"event_type":  scraper.EventGameEnd,
			"team_scores": teamScores,
			"scores":      scores,
		}
		endEvt := scraper.MakeEnvelope("event", name, tick, endPayload)
		broadcastEnvelope(endEvt, hub, pbClient)

		// Reset inter-tick state for next game.
		*state = *scraper.NewTickState()
	}

	broadcastSnapshot(name, tick, snap, hub, pbClient)
}

func broadcastSnapshot(name string, tick uint32, snap scraper.SnapshotPayload, hub *ws.Hub, pbClient *pb.Client) {
	env := scraper.MakeEnvelope("snapshot", name, tick, snap)
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

func broadcastEnvelope(env scraper.Envelope, hub *ws.Hub, pbClient *pb.Client) {
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

// ---------------------------------------------------------------------------
// Container API endpoints
// ---------------------------------------------------------------------------

func registerContainerAPI(hub *ws.Hub, mgr *podman.Manager) {
	// GET /api/containers — list all containers with status.
	// POST /api/containers — create a new container pair.
	hub.Handle("/api/containers", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			return
		}

		switch r.Method {
		case http.MethodGet:
			list, err := mgr.List()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(list)

		case http.MethodPost:
			var body struct {
				Name string `json:"name"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
				http.Error(w, "name is required", http.StatusBadRequest)
				return
			}
			info, err := mgr.Create(body.Name)
			if err != nil {
				http.Error(w, err.Error(), http.StatusConflict)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(info)

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Routes under /api/containers/{name}/...
	hub.Handle("/api/containers/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			return
		}

		// Parse: /api/containers/{name}[/action]
		path := strings.TrimPrefix(r.URL.Path, "/api/containers/")
		parts := strings.SplitN(path, "/", 2)
		name := parts[0]
		action := ""
		if len(parts) > 1 {
			action = parts[1]
		}

		switch {
		case r.Method == http.MethodPost && action == "start":
			if err := mgr.Start(name); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)

		case r.Method == http.MethodPost && action == "stop":
			if err := mgr.Stop(name); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)

		case r.Method == http.MethodDelete && action == "":
			if err := mgr.Remove(name); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)

		case r.Method == http.MethodGet && action == "":
			status, err := mgr.Status(name)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"status": status})

		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
}

// ---------------------------------------------------------------------------
// ringLog
// ---------------------------------------------------------------------------

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
