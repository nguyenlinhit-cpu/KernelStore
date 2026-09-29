// Package ws là chat realtime qua WebSocket: Hub giữ kết nối in-memory
// (tương đương ChatConnectionManager của C#) và endpoint /ws/chat.
//
// Lưu ý: vì kết nối nằm trong bộ nhớ, chỉ chạy được MỘT instance backend.
package ws

import (
	"context"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

// writeTimeout giới hạn thời gian gửi tới một kết nối, để client treo không chặn người gửi.
const writeTimeout = 5 * time.Second

// Hub: userID → (connectionID → kết nối). Một user có thể mở nhiều tab.
type Hub struct {
	mu    sync.RWMutex
	conns map[uuid.UUID]map[uuid.UUID]*websocket.Conn
}

func NewHub() *Hub {
	return &Hub{conns: make(map[uuid.UUID]map[uuid.UUID]*websocket.Conn)}
}

func (h *Hub) Add(userID, connID uuid.UUID, c *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.conns[userID] == nil {
		h.conns[userID] = make(map[uuid.UUID]*websocket.Conn)
	}
	h.conns[userID][connID] = c
}

func (h *Hub) Remove(userID, connID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.conns[userID], connID)
	if len(h.conns[userID]) == 0 {
		delete(h.conns, userID)
	}
}

// SendToUser gửi payload JSON tới mọi kết nối đang mở của user (offline thì bỏ qua).
// Kết nối gửi lỗi bị loại khỏi hub.
func (h *Hub) SendToUser(ctx context.Context, userID uuid.UUID, payload []byte) {
	h.mu.RLock()
	targets := make(map[uuid.UUID]*websocket.Conn, len(h.conns[userID]))
	for id, c := range h.conns[userID] {
		targets[id] = c
	}
	h.mu.RUnlock()

	for connID, c := range targets {
		wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
		err := c.Write(wctx, websocket.MessageText, payload)
		cancel()
		if err != nil {
			h.Remove(userID, connID)
		}
	}
}
