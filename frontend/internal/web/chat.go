package web

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/KernelStore/frontend/internal/i18n"
	"github.com/KernelStore/frontend/internal/views"
)

func (a *App) chatPage(w http.ResponseWriter, r *http.Request) {
	ctx, token := r.Context(), session(r).Token
	vm := views.ChatVM{Selected: r.URL.Query().Get("c"),
		WSURL: a.wsBase + "?access_token=" + url.QueryEscape(token)}
	// Mở hội thoại trước (server đánh dấu đã đọc) rồi mới nạp danh sách để số chưa đọc đúng.
	if vm.Selected != "" {
		vm.Messages, _ = a.api.GetMessages(ctx, token, vm.Selected)
	}
	vm.Conversations, _ = a.api.ListConversations(ctx, token)
	page(w, r, views.ChatPage(vm))
}

func (a *App) chatList(w http.ResponseWriter, r *http.Request) {
	list, _ := a.api.ListConversations(r.Context(), session(r).Token)
	fragment(w, r, views.ChatList(list, r.URL.Query().Get("c")))
}

// chatSend: gửi tin → trả bong bóng tin mới để chèn cuối khung chat, và báo làm mới danh sách.
// Lỗi → 422 (htmx không chèn gì, giữ nguyên nội dung ô nhập) + toast.
func (a *App) chatSend(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	content := strings.TrimSpace(r.FormValue("content"))
	if content == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	msg, err := a.api.SendMessage(ctx, session(r).Token, r.PathValue("id"), content)
	if err != nil {
		toast(w, toastError, i18n.Tc(ctx, "chat.send_failed")+err.Error())
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("HX-Trigger-After-Swap", "chat-refresh")
	fragment(w, r, views.ChatBubble(msg))
}
