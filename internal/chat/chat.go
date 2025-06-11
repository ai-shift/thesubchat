// Contains business logic for the chat interaction
package chat

import (
	"log/slog"
	"net/http"
	"shellshift/internal/templates"
)

type ChatHandler struct {
	templates *templates.Templates
}

func InitMux() *http.ServeMux {
	h := ChatHandler{
		templates: templates.New("internal/chat/views/*.html"),
	}
	m := http.NewServeMux()
	m.HandleFunc("GET /", h.getChat)
	return m
}

func (h ChatHandler) getChat(w http.ResponseWriter, r *http.Request) {
	err := h.templates.Render(w, "index", nil)
	if err != nil {
		slog.Error("failed to render index page", "with", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
