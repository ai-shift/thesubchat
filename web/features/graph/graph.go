// Provides structurized user's chats representation
package graph

import (
	"context"
	"log/slog"
	"net/http"

	"shellshift/internal/db"
	"shellshift/internal/templates"
	"shellshift/web"
)

type GraphHandler struct {
	chatURI string
	t       *templates.Templates
	q       *db.Queries
}

func InitMux(q *db.Queries, chatURI string) *http.ServeMux {
	h := GraphHandler{
		chatURI: chatURI,
		t:       templates.New("web/features/graph/views/*.html"),
		q:       q,
	}

	m := http.NewServeMux()
	m.HandleFunc("GET /", h.getGraph)
	return m
}

type Graph struct {
	ChatURI  string
	Graph    []any
	Keybinds web.KeybindsTable
}

func (h GraphHandler) getGraph(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	chats, err := h.q.FindChatTags(ctx)
	if err != nil {
		slog.Error("failed to find chats", "err", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	mentions, err := h.q.FindChatMentions(ctx)
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
