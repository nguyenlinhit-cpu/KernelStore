package handlers

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/KernelStore/backend-go/internal/dto"
	"github.com/KernelStore/backend-go/internal/httpx"
	"github.com/KernelStore/backend-go/internal/models"
	"github.com/KernelStore/backend-go/internal/repository"
	"github.com/KernelStore/backend-go/internal/services"
)

// ChatController: /api/chat — hội thoại giữa khách và shop; tin mới được đẩy realtime qua /ws/chat.
func (h *Handler) registerChat(rt *httpx.Router) {
	rt.Handle("GET /api/chat/conversations", httpx.Authenticated, h.listConversations)
	rt.Handle("POST /api/chat/conversations", httpx.Authenticated, h.startConversation)
	rt.Handle("GET /api/chat/conversations/{id}/messages", httpx.Authenticated, h.listMessages)
	rt.Handle("POST /api/chat/conversations/{id}/messages", httpx.Authenticated, h.sendMessage)
}

func (h *Handler) listConversations(w http.ResponseWriter, r *http.Request) {
	uid := userID(r)
	convos, err := repository.ListConversations(r.Context(), h.db, uid)
	if err != nil {
		serverError(w, r, err)
		return
	}
	out := make([]dto.ConversationDto, 0, len(convos))
	for i := range convos {
		out = append(out, conversationDto(&convos[i], uid, true))
	}
	dto.OK(w, out, "OK")
}

// startConversation: khách mở (hoặc lấy lại) hội thoại với một shop.
func (h *Handler) startConversation(w http.ResponseWriter, r *http.Request) {
	var req dto.StartConversationRequest
	if !httpx.BindJSON(w, r, &req) {
		return
	}
	ctx, uid := r.Context(), userID(r)

	shop, err := repository.FindShopByID(ctx, h.db, req.ShopID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if shop == nil {
		dto.NotFound(w, "Không tìm thấy shop")
		return
	}
	if shop.OwnerID == uid {
		dto.BadRequest(w, "Không thể chat với shop của chính bạn")
		return
	}
	id, err := repository.GetOrCreateConversation(ctx, h.db, uid, shop.ID, services.Now())
	if err != nil {
		serverError(w, r, err)
		return
	}
	convo, err := repository.FindConversation(ctx, h.db, uid, id)
	if err != nil || convo == nil {
		serverError(w, r, cmpErr(err, "hội thoại vừa tạo không tìm thấy"))
		return
	}
	dto.OK(w, conversationDto(convo, uid, false), "OK")
}

// listMessages: lịch sử tin nhắn, đồng thời đánh dấu đã đọc các tin của phía kia.
func (h *Handler) listMessages(w http.ResponseWriter, r *http.Request) {
	convo := h.participantConversation(w, r)
	if convo == nil {
		return
	}
	ctx := r.Context()
	messages, err := repository.ListMessages(ctx, h.db, convo.ID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if err := repository.MarkMessagesRead(ctx, h.db, convo.ID, userID(r)); err != nil {
		serverError(w, r, err)
		return
	}
	out := make([]dto.ChatMessageDto, 0, len(messages))
	for i := range messages {
		out = append(out, messageDto(&messages[i]))
	}
	dto.OK(w, out, "OK")
}

// sendMessage: lưu DB rồi đẩy realtime tới người còn lại qua WebSocket.
func (h *Handler) sendMessage(w http.ResponseWriter, r *http.Request) {
	var req dto.SendMessageRequest
	if !httpx.BindJSON(w, r, &req) {
		return
	}
	convo := h.participantConversation(w, r)
	if convo == nil {
		return
	}
	ctx, uid := r.Context(), userID(r)
	msg := &models.ChatMessage{
		ID: services.NewUUID(), ConversationID: convo.ID, SenderID: uid,
		Content: strings.TrimSpace(req.Content), CreatedAt: services.Now(),
	}
	if err := repository.InTx(ctx, h.db, func(tx pgx.Tx) error {
		return repository.InsertMessage(ctx, tx, msg)
	}); err != nil {
		serverError(w, r, err)
		return
	}

	recipient := convo.BuyerID
	if convo.BuyerID == uid {
		recipient = convo.ShopOwnerID
	}
	payload := messageDto(msg)
	if body, err := dto.MarshalJSON(payload); err == nil {
		h.hub.SendToUser(ctx, recipient, body)
	}
	dto.OK(w, payload, "OK")
}

// participantConversation: hội thoại {id} mà người gọi là người mua hoặc chủ shop.
// Không có → 404; không tham gia → 403.
func (h *Handler) participantConversation(w http.ResponseWriter, r *http.Request) *repository.ConversationRow {
	uid := userID(r)
	convo, err := repository.FindConversation(r.Context(), h.db, uid, httpx.PathGUID(r, "id"))
	if err != nil {
		serverError(w, r, err)
		return nil
	}
	if convo == nil {
		dto.NotFound(w, "Không tìm thấy hội thoại")
		return nil
	}
	if convo.BuyerID != uid && convo.ShopOwnerID != uid {
		dto.Forbidden(w, "Bạn không thuộc hội thoại này")
		return nil
	}
	return convo
}

// conversationDto: OtherName = tên shop với khách, = tên khách với chủ shop.
// withStats = false khi vừa mở hội thoại (C# trả LastMessage = null, UnreadCount = 0).
func conversationDto(c *repository.ConversationRow, viewer uuid.UUID, withStats bool) dto.ConversationDto {
	shopName, buyerName := deref(c.ShopName), deref(c.BuyerName)
	other := buyerName
	if c.BuyerID == viewer {
		other = shopName
	}
	d := dto.ConversationDto{
		ID: c.ID, ShopID: c.ShopID, ShopName: shopName, BuyerID: c.BuyerID, BuyerName: buyerName,
		OtherName: other, LastMessageAt: c.LastMessageAt,
	}
	if withStats {
		d.LastMessage, d.UnreadCount = c.LastMessage, c.UnreadCount
	}
	return d
}

func messageDto(m *models.ChatMessage) dto.ChatMessageDto {
	return dto.ChatMessageDto{ID: m.ID, ConversationID: m.ConversationID, SenderID: m.SenderID,
		Content: m.Content, CreatedAt: m.CreatedAt}
}
