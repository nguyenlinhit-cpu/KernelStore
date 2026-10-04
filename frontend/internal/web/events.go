package web

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Event đại diện cho một sự kiện realtime phát tới các client qua SSE.
type Event struct {
	Type    string         `json:"type"`   // "product-updated"
	Action  string         `json:"action"` // "create", "update", "delete"
	ID      string         `json:"id"`
	Slug    string         `json:"slug"`
	Payload map[string]any `json:"payload,omitempty"`
}

type EventBroker struct {
	mu      sync.RWMutex
	clients map[chan []byte]bool
}

func NewEventBroker() *EventBroker {
	return &EventBroker{
		clients: make(map[chan []byte]bool),
	}
}

func (b *EventBroker) Subscribe() chan []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan []byte, 32)
	b.clients[ch] = true
	return ch
}

func (b *EventBroker) Unsubscribe(ch chan []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.clients, ch)
	close(ch)
}

func (b *EventBroker) Broadcast(ev Event) {
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

// sseEvents phục vụ kết nối Server-Sent Events cho trình duyệt tại /events.
func (a *App) sseEvents(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := a.events.Subscribe()
	defer a.events.Unsubscribe(ch)

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

// startBackendEventSync lắng nghe sự kiện từ backend /api/events và chuyển tiếp tới các trình duyệt.
func (a *App) startBackendEventSync(ctx context.Context, apiBase string) {
	// Chuyển /api sang /api/events
	eventsURL := strings.TrimRight(apiBase, "/") + "/events"

	client := &http.Client{Timeout: 0}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, eventsURL, nil)
			if err != nil {
				time.Sleep(1 * time.Second)
				continue
			}

			resp, err := client.Do(req)
			if err != nil || resp.StatusCode != http.StatusOK {
				if resp != nil {
					_ = resp.Body.Close()
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(1 * time.Second):
					continue
				}
			}

			slog.Info("Connected to backend SSE stream", "url", eventsURL)
			reader := bufio.NewReader(resp.Body)
			var currentEvent string

			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					_ = resp.Body.Close()
					break
				}
				line = strings.TrimRight(line, "\r\n")
				if strings.HasPrefix(line, "event: ") {
					currentEvent = strings.TrimPrefix(line, "event: ")
				} else if strings.HasPrefix(line, "data: ") {
					dataStr := strings.TrimPrefix(line, "data: ")
					var ev Event
					if json.Unmarshal([]byte(dataStr), &ev) == nil {
						if ev.Type == "" {
							ev.Type = currentEvent
						}
						a.events.Broadcast(ev)
					}
				} else if line == "" {
					currentEvent = ""
				}
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
	}()
}
