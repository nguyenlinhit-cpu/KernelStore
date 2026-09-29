package ws

import (
	"net/http"
	"strings"

	"github.com/coder/websocket"
	"github.com/google/uuid"

	"github.com/KernelStore/backend-go/internal/services"
)

// Handler phục vụ ws://host/ws/chat?access_token=JWT.
// Server chỉ đẩy tin xuống (client gửi tin qua REST); vòng đọc chỉ để giữ kết nối
// và phát hiện khi client ngắt.
func Handler(hub *Hub, tokens *services.TokenService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isWebSocketRequest(r) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		claims, err := tokens.ParseAccessToken(r.URL.Query().Get("access_token"))
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Như bản C# (UseWebSockets không đặt AllowedOrigins): nhận mọi Origin;
		// quyền truy cập đã được xác thực bằng access_token.
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return // Accept đã tự ghi response lỗi
		}
		conn.SetReadLimit(-1) // C# đọc theo từng khúc, không giới hạn kích thước tin

		connID := uuid.New()
		hub.Add(claims.UserID, connID, conn)
		defer hub.Remove(claims.UserID, connID)

		ctx := r.Context()
		for {
			// Bỏ qua nội dung client gửi; Read trả lỗi khi client đóng / mất kết nối.
			if _, _, err := conn.Read(ctx); err != nil {
				conn.Close(websocket.StatusNormalClosure, "bye")
				return
			}
		}
	}
}

// isWebSocketRequest = HttpContext.WebSockets.IsWebSocketRequest.
func isWebSocketRequest(r *http.Request) bool {
	return r.Method == http.MethodGet &&
		headerHasToken(r.Header.Values("Connection"), "upgrade") &&
		headerHasToken(r.Header.Values("Upgrade"), "websocket")
}

func headerHasToken(values []string, token string) bool {
	for _, v := range values {
		for part := range strings.SplitSeq(v, ",") {
			if strings.EqualFold(strings.TrimSpace(part), token) {
				return true
			}
		}
	}
	return false
}
