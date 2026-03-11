package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// wsClient tracks one connected WebSocket client and its subscription filter.
// subs is nil to mean "all instances"; otherwise it's a set of instance names.
type wsClient struct {
	conn *websocket.Conn
	subs map[string]struct{}
}

func (c *wsClient) wantsInstance(inst string) bool {
	if c.subs == nil {
		return true
	}
	_, ok := c.subs[inst]
	return ok
}

// Hub manages all connected WebSocket clients and broadcasts messages to them.
// It is safe for concurrent use from multiple goroutines.
type Hub struct {
	mu           sync.Mutex
	clients      map[*websocket.Conn]*wsClient
	lastSnapshot map[string][]byte // instance → most recent snapshot JSON
	instances    []string          // registered instance names, in order
	extraRoutes  []struct {
		pattern string
		handler http.Handler
	}
}

// NewHub creates a Hub ready for use.
func NewHub() *Hub {
	return &Hub{
		clients:      make(map[*websocket.Conn]*wsClient),
		lastSnapshot: make(map[string][]byte),
	}
}

// Handle registers an additional HTTP route on the server mux.
// Must be called before ListenAndServe.
func (h *Hub) Handle(pattern string, handler http.Handler) {
	h.extraRoutes = append(h.extraRoutes, struct {
		pattern string
		handler http.Handler
	}{pattern, handler})
}

// RegisterInstance adds inst to the hub's known instance list.
// Only successfully-initialized instances should be registered.
func (h *Hub) RegisterInstance(inst string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, n := range h.instances {
		if n == inst {
			return
		}
	}
	h.instances = append(h.instances, inst)
}

// UnregisterInstance removes an instance from the hub's list and clears its
// cached snapshot. Call when an instance goes offline.
func (h *Hub) UnregisterInstance(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i, n := range h.instances {
		if n == name {
			h.instances = append(h.instances[:i], h.instances[i+1:]...)
			break
		}
	}
	delete(h.lastSnapshot, name)
}

// ClientCount returns the number of currently connected WebSocket clients.
func (h *Hub) ClientCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}

// ServeHTTP routes /rooms to serveRooms and everything else to serveWS.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/rooms" {
		h.serveRooms(w, r)
		return
	}
	h.serveWS(w, r)
}

// serveRooms returns the list of registered instance names as JSON.
func (h *Hub) serveRooms(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	names := make([]string, len(h.instances))
	copy(names, h.instances)
	h.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	// CORS wildcard required when debug page is opened as a file:// URL.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if err := json.NewEncoder(w).Encode(names); err != nil {
		log.Printf("ws: rooms encode error: %v", err)
	}
}

// serveWS upgrades the connection to WebSocket, registers the client with its
// subscription filter, replays cached snapshots, then drains until disconnect.
func (h *Hub) serveWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws: upgrade error: %v", err)
		return
	}

	client := &wsClient{conn: conn, subs: parseSubs(r)}

	h.mu.Lock()
	h.clients[conn] = client
	// Replay cached snapshots filtered to this client's subscriptions.
	for inst, msg := range h.lastSnapshot {
		if client.wantsInstance(inst) {
			_ = conn.WriteMessage(websocket.TextMessage, msg)
		}
	}
	h.mu.Unlock()

	// Drain incoming messages (clients are receive-only; ignore content).
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}

	h.mu.Lock()
	delete(h.clients, conn)
	h.mu.Unlock()
	conn.Close()
}

// parseSubs parses the "host" query param into a subscription set.
// Returns nil if the param is absent (meaning "all instances").
// Accepts comma-separated values: ?host=xemu-host-01,xemu-host-02
func parseSubs(r *http.Request) map[string]struct{} {
	raw := r.URL.Query().Get("host")
	if raw == "" {
		return nil
	}
	subs := make(map[string]struct{})
	for _, p := range strings.Split(raw, ",") {
		if p = strings.TrimSpace(p); p != "" {
			subs[p] = struct{}{}
		}
	}
	if len(subs) == 0 {
		return nil
	}
	return subs
}

// Broadcast sends a raw JSON message to all clients subscribed to inst.
// Clients that fail to receive are removed silently.
func (h *Hub) Broadcast(inst string, msg []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for conn, client := range h.clients {
		if !client.wantsInstance(inst) {
			continue
		}
		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			delete(h.clients, conn)
			conn.Close()
		}
	}
}

// BroadcastJSON marshals v and broadcasts it to clients subscribed to inst.
func (h *Hub) BroadcastJSON(inst string, v any) {
	msg, err := json.Marshal(v)
	if err != nil {
		log.Printf("ws: marshal error: %v", err)
		return
	}
	h.Broadcast(inst, msg)
}

// BroadcastSnapshot broadcasts a snapshot envelope, caches it for new clients,
// and sends only to clients subscribed to the given instance.
func (h *Hub) BroadcastSnapshot(instance string, msg []byte) {
	h.mu.Lock()
	h.lastSnapshot[instance] = msg
	for conn, client := range h.clients {
		if !client.wantsInstance(instance) {
			continue
		}
		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			delete(h.clients, conn)
			conn.Close()
		}
	}
	h.mu.Unlock()
}

// ListenAndServe starts the WebSocket HTTP server on addr (e.g. ":9000").
func (h *Hub) ListenAndServe(addr string) error {
	mux := http.NewServeMux()
	mux.Handle("/ws", h)
	mux.Handle("/", h) // also accept bare connections and /rooms
	mux.Handle("/ui/", http.StripPrefix("/ui/", http.FileServer(http.Dir("web"))))
	for _, r := range h.extraRoutes {
		mux.Handle(r.pattern, r.handler)
	}
	log.Printf("ws: listening on %s", addr)
	return http.ListenAndServe(addr, mux)
}
