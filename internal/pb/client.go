package pb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

const queueDepth = 512

// Client posts records to a PocketBase instance via REST. Writes are fire-and-forget:
// a background goroutine drains a channel and performs the HTTP POST. If the
// channel is full (PocketBase unreachable) records are dropped with a log message.
type Client struct {
	baseURL string
	http    *http.Client
	ch      chan pbMsg
}

type pbMsg struct {
	collection string
	body       any
}

// NewClient creates a PocketBase client and starts its drain goroutine.
func NewClient(baseURL string) *Client {
	c := &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 5 * time.Second},
		ch:      make(chan pbMsg, queueDepth),
	}
	go c.drain()
	return c
}

func (c *Client) drain() {
	for msg := range c.ch {
		if err := c.post(msg.collection, msg.body); err != nil {
			log.Printf("pb: post %s: %v", msg.collection, err)
		}
	}
}

func (c *Client) post(collection string, body any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	url := fmt.Sprintf("%s/api/collections/%s/records", c.baseURL, collection)
	resp, err := c.http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

// send enqueues a message. Drops (logs) if the channel is full.
func (c *Client) send(collection string, body any) {
	select {
	case c.ch <- pbMsg{collection, body}:
	default:
		log.Printf("pb: channel full, dropping %s record", collection)
	}
}

// PostSnapshot persists a snapshot record.
func (c *Client) PostSnapshot(instance string, tick uint32, payload any) {
	c.send("snapshots", map[string]any{
		"instance": instance,
		"tick":     tick,
		"type":     "snapshot",
		"payload":  payload,
	})
}

// PostEvent persists an event record.
func (c *Client) PostEvent(instance string, tick uint32, eventType string, payload any) {
	c.send("events", map[string]any{
		"instance":   instance,
		"tick":       tick,
		"event_type": eventType,
		"payload":    payload,
	})
}
