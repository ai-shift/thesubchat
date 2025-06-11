// Contains business logic for the chat interaction
package chat

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"shellshift/internal/chat/llm"
	"shellshift/internal/db"
	"shellshift/internal/templates"
	"strconv"
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
	m.HandleFunc("POST /{id}/user/message", h.postUserMessage)
	return m
}

func (h ChatHandler) getChat(w http.ResponseWriter, r *http.Request) {
	slog.Info("getting chat")
	id, err := deserID(w, r)
	if err != nil {
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

func (h ChatHandler) postUserMessage(w http.ResponseWriter, r *http.Request) {
	// Get chat
	id, err := deserID(w, r)
	if err != nil {
		return
	}
	chat, err := findChat(h.q, id)
	if err != nil {
		slog.Error("failed to find chat", "err", err.Error())
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	// Eval prompt
	prompt := r.FormValue("prompt")
	chat.Messages = append(chat.Messages, llm.Message{Text: prompt, Role: "user"})
	chat.Messages, err = h.llm.Eval(chat.Messages)
	if err != nil {
		slog.Error("failed to get eval prompt", "prompt", prompt, "err", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	slog.Info("user prompt", "prompt", prompt)
	// Persist new messages
	encoded, err := json.Marshal(chat.Messages)
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
	err = h.templates.Render(w, "messages", chat.Messages)
	if err != nil {
		slog.Error("failed to render index page", "with", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func deserID(w http.ResponseWriter, r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		slog.Error("failed to parse chat id", "id", id)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return 0, err
	}
	return id, nil
}
