package authhttp

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/clerk/clerk-sdk-go/v2"
	clerkhttp "github.com/clerk/clerk-sdk-go/v2/http"
	"github.com/clerk/clerk-sdk-go/v2/jwt"
	"github.com/clerk/clerk-sdk-go/v2/user"

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
	protectedHandler := http.HandlerFunc(h.getProfile)
	m.HandleFunc("GET /login", h.getLogin)
	m.HandleFunc("GET /register", h.getRegister)
	m.Handle("GET /profile", ProtectedRoute(h.getProfile))
	m.Handle(
		"GET /clerk",
		clerkhttp.RequireHeaderAuthorization(
			clerkhttp.AuthorizationJWTExtractor(AuthorizationOrCookieJWTExtractor),
		)(protectedHandler),
	)
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
	_, ok := clerk.SessionClaimsFromContext(r.Context())
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"access": "unauthorized"}`))
		return
	}

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

func ProtectedRoute(next func(http.ResponseWriter, *http.Request)) http.Handler {

	inner := func(w http.ResponseWriter, r *http.Request) {
		session, err := r.Cookie("__session")
		if err != nil {
			http.Error(w, "Session cookie not found", http.StatusUnauthorized)
			return
		}

		// Verify the session
		claims, err := jwt.Verify(r.Context(), &jwt.VerifyParams{
			Token: session.Value,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		usr, err := user.Get(r.Context(), claims.Subject)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			slog.Error("Failed to get the user", "with", err)
			return
		}

		slog.Info("User found", "user", usr)

		next(w, r)
	}

	return clerkhttp.WithHeaderAuthorization()(http.HandlerFunc(inner))
}

func AuthorizationOrCookieJWTExtractor(r *http.Request) string {
	authorization := strings.TrimSpace(r.Header.Get("Authorization"))
	authorization = strings.TrimPrefix(authorization, "Bearer ")
	if authorization == "" {
		sessionCookie, err := r.Cookie("__session")
		if err != nil {
			authorization = ""
		} else {
			authorization = sessionCookie.Value
		}
	}
	return authorization
}
