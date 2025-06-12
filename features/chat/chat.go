// Contains business logic for the chat interaction
package chat

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"github.com/google/uuid"
	"log/slog"
	"net/http"
	"shellshift/features/chat/schats"
	"shellshift/internal/db"
	"shellshift/internal/sse"
	"shellshift/internal/templates"
	"strings"
)

type ChatHandler struct {
	templates *templates.Templates
	q         *db.Queries
	g         *genkit.Genkit
	sc        *schats.StreamedChats
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
		q:         q,
		g:         g,
		sc:        schats.New(),
	}
	m := http.NewServeMux()
	m.HandleFunc("GET /{id}", h.getChat)
	m.HandleFunc("GET /", h.getEmptyChat)
	m.HandleFunc("POST /{id}/message", h.postUserMessage)
	m.HandleFunc("GET /{id}/message", h.getMessageStream)
	m.HandleFunc("GET /{id}/tags", h.getTags)
	m.HandleFunc("POST /{id}/tags", h.postTags)
	m.HandleFunc("DELETE /{id}/tags", h.deleteTags)
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
		return
	}
}

func (h ChatHandler) getEmptyChat(w http.ResponseWriter, r *http.Request) {
	err := h.templates.Render(w, "index", Chat{
		ID: uuid.New(),
	})
	if err != nil {
		slog.Error("failed to render index page", "with", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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
			Messages: make([]Message, 0),
		}
		go func(ctx context.Context) {
			t, err := genTitle(ctx, h.g, prompt)
			if err != nil {
				errChan <- err
			} else {
				tChan <- t
			}
		}(r.Context())
	default:
		slog.Error("failed to find chat", "err", err.Error())
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	chat.Messages = append(chat.Messages, Message{Text: prompt, Role: "user"})

	// Eval prompt
	// TODO: Add timeout
	stream := h.sc.Alloc(chat.ID)
	go func(ctx context.Context, chat *Chat) {
		msg, err := generateMessage(ctx, h.g, chat.Messages, stream.Chunks)
		close(stream.Chunks)
		if err != nil {
			slog.Error("failed to generate message", "with", err)
			return
		}
		chat.Messages = append(chat.Messages, msg)
		saveChat(ctx, h.q, *chat)
		stream.Done <- struct{}{}
		slog.Error("message generation was finished")
	}(context.Background(), chat)

	if waitTitle {
		slog.Info("waiting for the title generation")
		select {
		case title := <-tChan:
			slog.Info("title generated")
			chat.Title = title
			// TODO: Save only title
			saveChat(r.Context(), h.q, *chat)
			slog.Info("Chat saved")
		case err := <-errChan:
			slog.Error("failed to generate title", "with", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Redirect to the new page
	if waitTitle {
		slog.Error("redirecting to the created chat")
		w.Header().Set("HX-Redirect", chat.ID.String())
		return
	}

	// Render messages
	err = h.templates.Render(w, "messages", chat)
	if err != nil {
		slog.Error("failed to render index page", "with", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h ChatHandler) getMessageStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	id, err := deserID(w, r)
	if err != nil {
		slog.Error("got invalid chat id", "val", id)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	stream, ok := h.sc.Get(id)
	if !ok {
		sse.Send(w, sse.Event{
			Type: "end",
			Data: "There is no stream",
		})
		return
	}

	msg := &Message{
		Role: "assistant",
	}

	for chunk := range stream.Chunks {
		msg.Text += chunk
		var tpl bytes.Buffer
		if err := h.templates.Render(&tpl, "message", msg); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		sse.Send(w, sse.Event{
			Type: "chunk",
			Data: strings.Replace(tpl.String(), "\n", "<br>", -1),
		})
	}

	<-stream.Done
	sse.Send(w, sse.Event{
		Type: "finished",
		Data: "",
	})

	<-r.Context().Done()
}

type ChatTags struct {
	ID   string
	Tags []Tag
}

type Tag struct {
	ChatID string
	Name   string
}

func (h ChatHandler) postTags(w http.ResponseWriter, r *http.Request) {
	slog.Info("Adding new tag")
	// Validate data
	id, err := deserID(w, r)
	if err != nil {
		return
	}
	tag, ok := deserTag(w, r)
	if !ok {
		return
	}

	// Persist tag
	err = h.q.SaveTag(r.Context(), db.SaveTagParams{
		ChatID: id.String(),
		Name:   tag,
	})
	switch err {
	case nil:
		break
	default:
		slog.Error("failed to save tag", "with", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Render new tag
	err = h.templates.Render(w, "tag", Tag{
		ChatID: id.String(),
		Name:   tag,
	})
	if err != nil {
		slog.Error("failed to render tag", "with", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h ChatHandler) getTags(w http.ResponseWriter, r *http.Request) {
	slog.Info("search for tags")
	// Validate data
	id, err := deserID(w, r)
	if err != nil {
		return
	}

	rows, err := h.q.FindTags(r.Context(), id.String())
	switch err {
	case nil:
		break
	case sql.ErrNoRows:
		break
	default:
		slog.Error("failed to find tags", "with", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tags := make([]Tag, len(rows))
	for i, name := range rows {
		tags[i] = Tag{
			ChatID: id.String(),
			Name:   name,
		}
	}

	err = h.templates.Render(w, "tags", ChatTags{
		ID:   id.String(),
		Tags: tags,
	})
	if err != nil {
		slog.Error("failed to render tempalte", "with", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h ChatHandler) deleteTags(w http.ResponseWriter, r *http.Request) {
	// Validate data
	id, err := deserID(w, r)
	if err != nil {
		return
	}
	tag, ok := deserTag(w, r)
	if !ok {
		return
	}

	// Delete tag
	err = h.q.DeleteTag(r.Context(), db.DeleteTagParams{
		ChatID: id.String(),
		Name:   tag,
	})
	if err != nil {
		slog.Error("failed to delete tag", "tag", tag, "with", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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

func deserTag(w http.ResponseWriter, r *http.Request) (tag string, ok bool) {
	tag = r.FormValue("tag")
	if tag == "" {
		http.Error(w, "Tag can not be empty", http.StatusBadRequest)
		return
	}
	if len(tag) > 30 {
		http.Error(w, "Tag should not be larger than 30 chars", http.StatusBadRequest)
		return
	}
	return tag, true
}
