package authhttp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/jwks"
	clerkjwt "github.com/clerk/clerk-sdk-go/v2/jwt"
	"github.com/clerk/clerk-sdk-go/v2/user"
	"github.com/go-jose/go-jose/v3/jwt"

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

	jwkStore := NewJWKStore()
	config := &clerk.ClientConfig{}
	config.Key = clerk.String(os.Getenv("CLERK_SECRET_KEY"))
	jwksClient := jwks.NewClient(config)

	m := http.NewServeMux()
	m.HandleFunc("GET /login", h.getLogin)
	m.HandleFunc("GET /register", h.getRegister)
	m.HandleFunc("GET /profile", h.protectedRoute(jwksClient, jwkStore))
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

type sessionRefresh struct {
	JWT string `json:"jwt"`
}

// TODO: Use JWK
// TODO: Call clerkjwt.Verify only once
// TODO: Rewrite into middleware
func (h AuthHandler) protectedRoute(jwksClient *jwks.Client, store JWKStore) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get the session JWT from the Authorization header
		sessionCookie, err := r.Cookie("__session")
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		sessionToken := sessionCookie.Value

		// Decode the session JWT so that we can find the key ID.
		unsafeClaims, err := clerkjwt.Decode(r.Context(), &clerkjwt.DecodeParams{
			Token: sessionToken,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		slog.Info("unsafe claims", "val", unsafeClaims)

		// Verify the session
		claims, err := clerkjwt.Verify(r.Context(), &clerkjwt.VerifyParams{
			Token: sessionToken,
		})
		switch err {
		default:
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		case nil:
			break
		case jwt.ErrExpired:
			slog.Info("refreshing session", "err", err)
			body := []byte("{}")
			url := fmt.Sprintf("https://api.clerk.com/v1/sessions/%s/tokens", unsafeClaims.Extra["sid"])
			client := &http.Client{}
			req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			req.Header.Add("Content-Type", "application/json")
			// TODO: Use handler state for SK
			req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("CLERK_SECRET_KEY")))

			resp, err := client.Do(req)
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}
			defer resp.Body.Close()

			body, err = io.ReadAll(resp.Body)
			if err != nil {
				slog.Error("failed to read response from clerk")
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			if resp.StatusCode >= 400 {
				slog.Info("failed to refresh token", "status", resp.StatusCode, "body", string(body), "url", url)
				http.Error(w, string(body), resp.StatusCode)
				return
			}

			var out sessionRefresh
			err = json.Unmarshal(body, &out)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			slog.Info("refreshed session", "body", out)
			sessionCookie.Value = out.JWT
			sessionCookie.Expires = sessionCookie.Expires.Add(60 * time.Second)
			http.SetCookie(w, sessionCookie)

			claims, err = clerkjwt.Verify(r.Context(), &clerkjwt.VerifyParams{
				Token: out.JWT,
			})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		usr, err := user.Get(r.Context(), claims.Subject)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		slog.Info("fetched user", "val", usr)

		err = h.t.Render(w, "profile", RegisterRender{
			LoginURI: h.loginURI,
			HomeURI:  h.homeURI,
		})

		if err != nil {
			slog.Error("failed to render PROFILE", "with", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

// Sample interface for JSON Web Key storage.
// Implementation may vary.
type JWKStore interface {
	GetJWK() *clerk.JSONWebKey
	SetJWK(*clerk.JSONWebKey)
}

type MemJWKStore struct {
	key *clerk.JSONWebKey
}

func (s MemJWKStore) GetJWK() *clerk.JSONWebKey {
	return s.key
}

func (s *MemJWKStore) SetJWK(key *clerk.JSONWebKey) {
	s.key = key
}

func NewJWKStore() JWKStore {
	// Implementation may vary. This can be an
	// in-memory store, database, caching layer,...
	return &MemJWKStore{}
}
