package auth

import (
	"log/slog"
	"net/http"
	"shellshift/internal/db"
	"shellshift/internal/templates"
)

type AuthHandler struct {
	t *templates.Templates
	q *db.Queries
}

func InitMux(q *db.Queries) *http.ServeMux {
	h := AuthHandler{
		t: templates.New("web/features/auth/views/*.html"),
		q: q,
	}

	m := http.NewServeMux()
	m.HandleFunc("GET /login", h.getLogin)
	return m
}

func (h AuthHandler) getLogin(w http.ResponseWriter, r *http.Request) {
	err := h.t.Render(w, "index", struct{}{})

	if err != nil {
		slog.Error("failed to render LOGIN", "with", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
