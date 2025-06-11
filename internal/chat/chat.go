// Contains business logic for the chat interaction
package chat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"shellshift/internal/chat/llm"
	"shellshift/internal/db"
	"shellshift/internal/templates"
)

type ChatHandler struct {
	ctx       context.Context
	templates *templates.Templates
	llm       *llm.LLM
	q         *db.Queries
}

func InitMux(q *db.Queries) *http.ServeMux {
	ctx := context.Background()
	h := ChatHandler{
		ctx:       ctx,
		templates: templates.New("internal/chat/views/*.html"),
		llm:       llm.New(ctx),
		q:         q,
	}
	m := http.NewServeMux()
	m.HandleFunc("GET /", h.getChat)
	m.HandleFunc("POST /user/message", h.postUserMessage)
	return m
}

func (h ChatHandler) getChat(w http.ResponseWriter, r *http.Request) {
	err := h.templates.Render(w, "index", nil)
	if err != nil {
		slog.Error("failed to render index page", "with", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h ChatHandler) postUserMessage(w http.ResponseWriter, r *http.Request) {
	// Get chat
	chat, err := h.q.FindChat(h.ctx, 1)
	switch err {
	case sql.ErrNoRows:
		slog.Info("creating new chat")
		chat.Messages = []byte("[]")
		chat.Title = "Test title"
	case nil:
		slog.Info("found chat", "title", chat.Title)
	default:
		slog.Error("failed to find chat", "err", fmt.Sprintf("%#v", err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var msgs []llm.Message
	err = json.Unmarshal(chat.Messages, &msgs)
	if err != nil {
		slog.Error("failed to decode chat messages", "err", err.Error())
		return
	}
	// Eval prompt
	prompt := r.FormValue("prompt")
	msgs = append(msgs, llm.Message{Text: prompt, Role: "user"})
	msgs, err = h.llm.Eval(msgs)
	if err != nil {
		slog.Error("failed to get eval prompt", "prompt", prompt, "err", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	slog.Info("user prompt", "prompt", prompt)
	// Persist new messages
	encoded, err := json.Marshal(msgs)
	if err != nil {
		slog.Error("failed to encode messages", "err", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = h.q.SaveChat(h.ctx, db.SaveChatParams{
		ID:       1,
		Title:    chat.Title,
		Messages: encoded,
	})
	if err != nil {
		slog.Error("failed to save chat", "err", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Render messages
	err = h.templates.Render(w, "messages", msgs)
	if err != nil {
		slog.Error("failed to render index page", "with", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

}
