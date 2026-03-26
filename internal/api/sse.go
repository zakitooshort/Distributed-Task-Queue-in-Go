package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
)

// Event is what gets sent to dashboard clients over SSE
type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// client represents one connected dashboard tab
type client struct {
	ch chan Event
}

// Broadcaster manages all connected SSE clients
// safe for concurrent use — all access goes through the mutex
type Broadcaster struct {
	mu      sync.RWMutex
	clients map[*client]struct{}
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		clients: make(map[*client]struct{}),
	}
}

// Register adds a new SSE client and returns their channel
func (b *Broadcaster) Register() (chan Event, func()) {
	c := &client{ch: make(chan Event, 32)} // buffered so slow clients don't block us

	b.mu.Lock()
	b.clients[c] = struct{}{}
	b.mu.Unlock()

	slog.Debug("sse client connected", "total", b.ClientCount())

	// return the channel + a cleanup function
	cleanup := func() {
		b.mu.Lock()
		delete(b.clients, c)
		close(c.ch)
		b.mu.Unlock()
		slog.Debug("sse client disconnected", "total", b.ClientCount())
	}

	return c.ch, cleanup
}

// Broadcast sends an event to all connected clients
// implements the worker.Broadcaster interface too
func (b *Broadcaster) Broadcast(eventType string, data any) {
	event := Event{Type: eventType, Data: data}

	b.mu.RLock()
	defer b.mu.RUnlock()

	for c := range b.clients {
		select {
		case c.ch <- event:
		default:
			// client is too slow / buffer full — skip it
			// it'll catch up on the next event or reconnect
		}
	}
}

// ClientCount returns how many clients are currently connected
func (b *Broadcaster) ClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}

// FormatSSE formats an event in the SSE wire format:
// data: {...}\n\n
func FormatSSE(event Event) (string, error) {
	b, err := json.Marshal(event)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("data: %s\n\n", b), nil
}
