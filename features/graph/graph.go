// Provides structurized user's chats representation
package graph

import (
	"context"
	"log/slog"
	"net/http"
	"shellshift/internal/db"
	"shellshift/internal/templates"
)

type GraphHandler struct {
	chatURI string
	t       *templates.Templates
	q       *db.Queries
}

func InitMux(q *db.Queries, chatURI string) *http.ServeMux {
	h := GraphHandler{
		chatURI: chatURI,
		t:       templates.New("features/graph/views/*.html"),
		q:       q,
	}

	m := http.NewServeMux()
	m.HandleFunc("GET /", h.getGraph)
	return m
}

type Graph struct {
	ChatURI string
	Chats   []db.GetGraphRow
}

func (h GraphHandler) getGraph(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	chats, err := h.q.GetGraph(ctx)
	if err != nil {
		slog.Error("failed to find chat", "err", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = h.t.Render(w, "index", Graph{
		ChatURI: h.chatURI,
		Chats:   chats,
	})

	if err != nil {
		slog.Error("failed to render graph", "with", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
