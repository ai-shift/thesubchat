package authhttp

import (
	"log/slog"
	"net/http"
	"shellshift/internal/auth"
	"shellshift/internal/db"
	"shellshift/internal/templates"
)

type AuthHandler struct {
	t           *templates.Templates
	q           *db.Queries
	loginURI    string
	registerURI string
	homeURI     string
}

func InitMux(q *db.Queries, loginURI, registerURI, homeURI string) *http.ServeMux {
	h := AuthHandler{
		t:           templates.New("web/features/authhttp/views/*.html"),
		q:           q,
		loginURI:    loginURI,
		registerURI: registerURI,
		homeURI:     homeURI,
	}

	m := http.NewServeMux()

	m.HandleFunc("GET /login", h.getLogin)
	m.HandleFunc("GET /register", h.getRegister)
	m.Handle("GET /profile", auth.ProtectedRoute(h.getProfile))
	return m
}

type LoginRender struct {
	RegisterURI string
	HomeURI     string
}

func (h AuthHandler) getLogin(w http.ResponseWriter, r *http.Request) {
	err := h.t.Render(w, "login", LoginRender{
		RegisterURI: h.registerURI,
		HomeURI:     h.homeURI,
	})

	if err != nil {
		slog.Error("failed to render LOGIN", "with", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

type RegisterRender struct {
	LoginURI string
	HomeURI  string
}

func (h AuthHandler) getRegister(w http.ResponseWriter, r *http.Request) {
	err := h.t.Render(w, "register", RegisterRender{
		LoginURI: h.loginURI,
		HomeURI:  h.homeURI,
	})

	if err != nil {
		slog.Error("failed to render REGISTER", "with", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h AuthHandler) getProfile(w http.ResponseWriter, r *http.Request) {
	err := h.t.Render(w, "profile", RegisterRender{
		LoginURI: h.loginURI,
		HomeURI:  h.homeURI,
	})

	if err != nil {
		slog.Error("failed to render PROFILE", "with", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
