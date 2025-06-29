package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
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
	t       *templates.Templates
	q       *db.Queries
	clerkSK string
	baseURI string
	homeURI string
}

func InitMux(q *db.Queries, protector *ProtectionMiddleware, clerkSK, baseURI, homeURI string) *http.ServeMux {
	h := AuthHandler{
		t:       templates.New("web/features/auth/views/*.html"),
		q:       q,
		clerkSK: clerkSK,
		baseURI: baseURI,
		homeURI: homeURI,
	}

	m := http.NewServeMux()
	m.HandleFunc("GET /login", h.getLogin)
	m.HandleFunc("GET /logout", protector.Protect(h.getLogout))
	m.HandleFunc("GET /register", h.getRegister)
	m.HandleFunc("GET /profile", protector.Protect(h.getProfile))
	return m
}

type LoginRender struct {
	BaseURI string
	HomeURI string
}

func (h AuthHandler) getLogin(w http.ResponseWriter, r *http.Request) {
	err := h.t.Render(w, "login", LoginRender{
		BaseURI: h.baseURI,
		HomeURI: h.homeURI,
	})

	if err != nil {
		slog.Error("failed to render LOGIN", "with", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h AuthHandler) getLogout(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("__session")
	redirect := func() {
		http.Redirect(w, r, fmt.Sprintf("%s/login", h.baseURI), http.StatusMovedPermanently)
	}

	if err != nil {
		redirect()
		return
	}

	unsafeClaims, err := clerkjwt.Decode(r.Context(), &clerkjwt.DecodeParams{
		Token: c.Value,
	})
	if err != nil {
		slog.Error("failed to decode JWT claims")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	url := fmt.Sprintf("https://api.clerk.com/v1/sessions/%s/revoke", unsafeClaims.Extra["sid"])
	client := &http.Client{}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte{}))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", h.clerkSK))

	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	c.Value = ""
	c.MaxAge = -1
	http.SetCookie(w, c)

	redirect()
}

type RegisterRender struct {
	BaseURI string
	HomeURI string
}

func (h AuthHandler) getRegister(w http.ResponseWriter, r *http.Request) {
	err := h.t.Render(w, "register", RegisterRender{
		BaseURI: h.baseURI,
		HomeURI: h.homeURI,
	})

	if err != nil {
		slog.Error("failed to render REGISTER", "with", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h AuthHandler) getProfile(w http.ResponseWriter, r *http.Request) {
	err := h.t.Render(w, "profile", RegisterRender{
		BaseURI: h.baseURI,
		HomeURI: h.homeURI,
	})

	if err != nil {
		slog.Error("failed to render PROFILE", "with", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

type ProtectionMiddleware struct {
	clerkSK    string
	jwk        *clerk.JSONWebKey
	jwksClient *jwks.Client
	baseURI    string
}

func NewProtectionMiddleware(baseURI, clerkSK string) *ProtectionMiddleware {
	config := &clerk.ClientConfig{}
	config.Key = clerk.String(clerkSK)

	return &ProtectionMiddleware{
		clerkSK:    clerkSK,
		jwksClient: jwks.NewClient(config),
		baseURI:    baseURI,
	}
}

type sessionRefresh struct {
	JWT string `json:"jwt"`
}

type userIDKey string

const UserIDKey userIDKey = "user_id"

func (m *ProtectionMiddleware) Protect(next func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	slog.Info("protecting route")
	return func(w http.ResponseWriter, r *http.Request) {
		slog.Info("check auth")
		// Get the session JWT from the Authorization header
		sessionCookie, err := r.Cookie("__session")
		if err != nil {
			slog.Info("session cookies wasn't found", "with", err)
			http.Redirect(w, r, fmt.Sprintf("%s/login", m.baseURI), http.StatusMovedPermanently)
			return
		}

		// Decode the session JWT so that we can find the key ID.
		unsafeClaims, err := clerkjwt.Decode(r.Context(), &clerkjwt.DecodeParams{
			Token: sessionCookie.Value,
		})
		if err != nil {
			slog.Error("failed to decode JWT claims")
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		// Retrieve JSON web key
		if m.jwk == nil {
			jwk, err := clerkjwt.GetJSONWebKey(r.Context(), &clerkjwt.GetJSONWebKeyParams{
				KeyID:      unsafeClaims.KeyID,
				JWKSClient: m.jwksClient,
			})
			if err != nil {
				slog.Error("Error while getting JWK", "with", err)
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}
			m.jwk = jwk
		}

		// Refresh token in needed
		claims, err := clerkjwt.Verify(r.Context(), &clerkjwt.VerifyParams{
			Token: sessionCookie.Value,
			JWK:   m.jwk,
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
			req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", m.clerkSK))

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
				// http.Redirect(w, r, fmt.Sprintf("%s/login", m.baseURI), http.StatusMovedPermanently)
				return
			}

			var out sessionRefresh
			err = json.Unmarshal(body, &out)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			sessionCookie.Value = out.JWT
			sessionCookie.Path = "/"
			sessionCookie.Expires = time.Now().Add(60 * time.Second)
			http.SetCookie(w, sessionCookie)
			slog.Info("cookie was updated")

			claims, err = clerkjwt.Verify(r.Context(), &clerkjwt.VerifyParams{
				Token: sessionCookie.Value,
				JWK:   m.jwk,
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
		r = r.WithContext(context.WithValue(r.Context(), UserIDKey, usr.ID))

		slog.Info("calling next")
		next(w, r)
	}
}
