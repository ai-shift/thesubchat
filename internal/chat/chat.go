// Contains business logic for the chat interaction
package chat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
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
	m.HandleFunc("GET /{id}", h.getChat)
	m.HandleFunc("GET /", h.getEmptyChat)
	m.HandleFunc("POST /{id}/user/message", h.postUserMessage)
	return m
}

func (h ChatHandler) getChat(w http.ResponseWriter, r *http.Request) {
	slog.Info("getting chat")
	id, err := deserID(w, r)
	if err != nil {
		slog.Error("failed to parse chat id", "with", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	chat, err := findChat(h.q, id)
	if err != nil {
		slog.Error("failed to find chat", "err", err.Error())
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	err = h.templates.Render(w, "index", chat)
	if err != nil {
		slog.Error("failed to render index page", "with", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h ChatHandler) getEmptyChat(w http.ResponseWriter, r *http.Request) {
	err := h.templates.Render(w, "index", Chat{Title: "New chat", ID: uuid.New()})
	if err != nil {
		slog.Error("failed to render index page", "with", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h ChatHandler) postUserMessage(w http.ResponseWriter, r *http.Request) {
	// Validate request
	prompt := r.FormValue("prompt")
	if prompt == "" {
		http.Error(w, "Prompt shouldn't be empty", http.StatusBadRequest)
		return
	}
	id, err := deserID(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Get chat
	chat, err := findChat(h.q, id)
	switch err {
	case nil:
		break
	case sql.ErrNoRows:
		chat = &Chat{
			ID:       id,
			Messages: make([]llm.Message, 0),
		}
	default:
		slog.Error("failed to find chat", "err", err.Error())
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Eval prompt
	chat.Messages, err = h.llm.Eval(append(chat.Messages, llm.Message{Text: prompt, Role: "user"}))
	if err != nil {
		slog.Error("failed to get eval prompt", "prompt", prompt, "err", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Persist new messages
	encoded, err := json.Marshal(chat.Messages)
	if err != nil {
		slog.Error("failed to encode messages", "err", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = h.q.UpdateChat(h.ctx, db.UpdateChatParams{
		ID:       id.String(),
		Messages: encoded,
	})
	if err != nil {
		slog.Error("failed to save chat", "err", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Render messages
	err = h.templates.Render(w, "messages", chat.Messages)
	if err != nil {
		slog.Error("failed to render index page", "with", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func deserID(w http.ResponseWriter, r *http.Request) (id uuid.UUID, err error) {
	id, err = uuid.Parse(r.PathValue("id"))
	if err != nil {
		slog.Error("failed to parse chat id", "id", id)
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	return
}
