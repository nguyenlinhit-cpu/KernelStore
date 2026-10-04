package events

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type Event struct {
	Type    string         `json:"type"`   // "product-updated"
	Action  string         `json:"action"` // "create", "update", "delete"
	ID      string         `json:"id"`
	Slug    string         `json:"slug"`
	Payload map[string]any `json:"payload,omitempty"`
}

type Broker struct {
	mu      sync.RWMutex
	clients map[chan []byte]bool
}

func NewBroker() *Broker {
	return &Broker{
		clients: make(map[chan []byte]bool),
	}
}

func (b *Broker) Subscribe() chan []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan []byte, 32)
	b.clients[ch] = true
	return ch
}

func (b *Broker) Unsubscribe(ch chan []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.clients, ch)
	close(ch)
}

func (b *Broker) Broadcast(ev Event) {
	data, err := json.Marshal(ev)
	if err != nil {
		return
	}
	msg := []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", ev.Type, data))

	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.clients {
		select {
		case ch <- msg:
		default:
		}
	}
}

func (b *Broker) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rc := http.NewResponseController(w)
		_ = rc.SetWriteDeadline(time.Time{})

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		ch := b.Subscribe()
		defer b.Unsubscribe(ch)

		fmt.Fprintf(w, ": connected\n\n")
		_ = rc.Flush()

		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = rc.SetWriteDeadline(time.Time{})
				_, _ = w.Write([]byte(": ping\n\n"))
				_ = rc.Flush()
			case msg, ok := <-ch:
				if !ok {
					return
				}
				_ = rc.SetWriteDeadline(time.Time{})
				_, _ = w.Write(msg)
				_ = rc.Flush()
			}
		}
	}
}
