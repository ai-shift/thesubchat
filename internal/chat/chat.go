// Contains business logic for the chat interaction
package chat

import (
	"context"
	"log/slog"
	"net/http"
	"shellshift/internal/chat/llm"
	"shellshift/internal/templates"
)

type ChatHandler struct {
	templates *templates.Templates
	llm       *llm.LLM
}

func InitMux() *http.ServeMux {
	ctx := context.Background()
	h := ChatHandler{
		templates: templates.New("internal/chat/views/*.html"),
		llm:       llm.New(ctx),
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
	prompt := r.FormValue("prompt")
	resp, err := h.llm.Eval(prompt)
	if err != nil {
		slog.Error("failed to get eval prompt", "prompt", prompt, "err", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	slog.Info("user prompt", "prompt", prompt)
	err = h.templates.Render(w, "message", resp)
	if err != nil {
		slog.Error("failed to render index page", "with", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

}
