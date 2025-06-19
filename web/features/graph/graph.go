// Provides structurized user's chats representation
package graph

import (
	"log/slog"
	"net/http"

	"shellshift/internal/db"
	"shellshift/internal/templates"
	"shellshift/web"
	"shellshift/web/features/auth"
)

type GraphHandler struct {
	chatURI string
	t       *templates.Templates
	db      *db.Factory
}

func InitMux(dbF *db.Factory, protector *auth.ProtectionMiddleware, chatURI string) *http.ServeMux {
	h := GraphHandler{
		chatURI: chatURI,
		t:       templates.New("web/features/graph/views/*.html"),
		db:      dbF,
	}

	m := http.NewServeMux()
	m.HandleFunc("GET /", protector.Protect(h.getGraph))
	return m
}

type Graph struct {
	ChatURI  string
	Graph    []any
	Keybinds web.KeybindsTable
}

func (h GraphHandler) getGraph(w http.ResponseWriter, r *http.Request) {
	q, err := h.getQueries(w, r)
	if err != nil {
		return
	}

	chats, err := q.FindChatTags(r.Context())
	if err != nil {
		slog.Error("failed to find chats", "err", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	mentions, err := q.FindChatMentions(r.Context())
	if err != nil {
		slog.Error("failed to find chat mentions", "err", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = h.t.Render(w, "index", Graph{
		ChatURI:  h.chatURI,
		Graph:    buildGraph(chats, mentions),
		Keybinds: web.Keybinds,
	})

	if err != nil {
		slog.Error("failed to render graph", "with", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h GraphHandler) getQueries(w http.ResponseWriter, r *http.Request) (*db.Queries, error) {
	userID := r.Context().Value(auth.UserIDKey)
	q, err := h.db.Get(userID.(string))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return q, err
}
