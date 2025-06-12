// Contains business logic for the chat interaction
package chat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"github.com/google/uuid"
	"log/slog"
	"net/http"
	"shellshift/features/chat/llm"
	"shellshift/internal/db"
	"shellshift/internal/templates"
)

type ChatHandler struct {
	templates *templates.Templates
	llm       *llm.LLM
	q         *db.Queries
	g         *genkit.Genkit
}

func InitMux(q *db.Queries) *http.ServeMux {
	ctx := context.Background()
	g, err := genkit.Init(ctx,
		genkit.WithPlugins(&googlegenai.GoogleAI{}),
		genkit.WithDefaultModel("googleai/gemini-2.0-flash"),
	)
	if err != nil {
		panic(fmt.Sprintf("could not initialize Genkit: %v", err))
	}
	h := ChatHandler{
		templates: templates.New("features/chat/views/*.html"),
		llm:       llm.New(ctx),
		q:         q,
		g:         g,
	}
	m := http.NewServeMux()
	m.HandleFunc("GET /{id}", h.getChat)
	m.HandleFunc("GET /", h.getEmptyChat)
	m.HandleFunc("POST /{id}/user/message", h.postUserMessage)
	return m
}

func (h ChatHandler) getChat(w http.ResponseWriter, r *http.Request) {
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Maybe title generation
	errChan := make(chan error)
	tChan := make(chan string)
	var waitTitle bool

	// Get chat
	chat, err := findChat(h.q, id)
	switch err {
	case nil:
		break
	case sql.ErrNoRows:
		waitTitle = true
		chat = &Chat{
			ID:       id,
			Messages: make([]llm.Message, 0),
		}
		go func() {
			t, err := genTitle(ctx, h.g, prompt)
			if err != nil {
				errChan <- err
			} else {
				tChan <- t
			}
		}()
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

	if waitTitle {
		slog.Info("waiting for the title generation")
		select {
		case title := <-tChan:
			slog.Info("title generated")
			chat.Title = title
		case err := <-errChan:
			slog.Error("failed to generate title", "with", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	slog.Info("saving chat", "id", chat.ID)
	err = h.q.SaveChat(ctx, db.SaveChatParams{
		ID:       chat.ID.String(),
		Title:    chat.Title,
		Messages: encoded,
	})
	if err != nil {
		slog.Error("failed to save chat", "err", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Redirect to the new page
	if waitTitle {
		slog.Error("redirecting to the created chat")
		w.Header().Set("HX-Redirect", chat.ID.String())
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
		slog.Error("failed to parse chat", "id", id, "with", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	return
}
