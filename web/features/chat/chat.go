// Contains business logic for the chat interaction
package chat

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"shellshift/internal/db"
	"shellshift/internal/sse"
	"shellshift/internal/templates"
	"shellshift/web/features/chat/textchan"
	"slices"
	"strings"

	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"github.com/google/uuid"

	"shellshift/web"
)

type ChatHandler struct {
	templates *templates.Templates
	q         *db.Queries
	g         *genkit.Genkit
	msgChan   *textchan.TextChan
	titleChan *textchan.TextChan
	baseURI   string
	graphURI  string
}

func InitMux(q *db.Queries, baseURI, graphURI string) *http.ServeMux {
	ctx := context.Background()
	g, err := genkit.Init(ctx,
		genkit.WithPlugins(&googlegenai.GoogleAI{}),
		genkit.WithDefaultModel("googleai/gemini-2.0-flash"),
	)
	if err != nil {
		panic(fmt.Sprintf("could not initialize Genkit: %v", err))
	}
	h := ChatHandler{
		templates: templates.New("web/features/chat/views/*.html"),
		q:         q,
		g:         g,
		msgChan:   textchan.New(),
		titleChan: textchan.New(),
		baseURI:   baseURI,
		graphURI:  graphURI,
	}
	m := http.NewServeMux()
	m.HandleFunc("GET /", h.getEmptyChat)
	m.HandleFunc("GET /{id}", h.getChat)
	m.HandleFunc("GET /{id}/branch/{branchId}", h.getChat)
	m.HandleFunc("DELETE /{id}", h.deleteChat)
	m.HandleFunc("POST /{id}/branch/{branchId}/message", h.postUserMessage)
	m.HandleFunc("GET /{id}/branch/{branchId}/message/stream", h.getMessageStream)
	m.HandleFunc("GET /{id}/branch/{branchId}/merge", h.getMerge)
	m.HandleFunc("GET /{id}/title", h.getTitle)
	m.HandleFunc("GET /{id}/tags", h.getTags)
	m.HandleFunc("POST /{id}/tags", h.postTags)
	m.HandleFunc("DELETE /{id}/tags", h.deleteTags)
	return m
}

type ChatRender struct {
	ID       uuid.UUID
	Title    string
	Messages []HTMLMessage
}

type ChatViewData struct {
	Chat              ChatRender
	Branch            Branch
	TitleGenerating   bool
	ChatTitles        []db.FindChatTitlesRow
	Keybinds          web.KeybindsTable
	BaseURI           string
	GraphURI          string
	MessageGenerating bool
}

// TODO: Add streaming message to new chat response
func (h ChatHandler) getChat(w http.ResponseWriter, r *http.Request) {
	id, err := deserID(w, r)
	if err != nil {
		slog.Error("failed to parse chat id", "with", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	branchID, exists, err := deserBranchID(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var branch Branch
	if exists {
		branch, err = findChatBranch(r.Context(), h.q, id, branchID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	} else {
		branch = Branch{ID: uuid.New()}
	}
	chat, err := findChat(r.Context(), h.q, id)
	if err != nil {
		slog.Error("failed to find chat", "err", err.Error())
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	chatTitles, err := findChatsTitles(h.q)

	if err != nil {
		slog.Error("failed to find chat titles", "err", err.Error())
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	_, titleGenerating := h.titleChan.Get(id)
	_, messageGenerating := h.msgChan.Get(branch.ID)
	err = h.templates.Render(w, "index", ChatViewData{
		Chat: ChatRender{
			ID:       chat.ID,
			Title:    chat.Title,
			Messages: renderMessages(chat),
		},
		Branch:            branch,
		TitleGenerating:   titleGenerating,
		ChatTitles:        chatTitles,
		Keybinds:          web.Keybinds,
		BaseURI:           h.baseURI,
		GraphURI:          h.graphURI,
		MessageGenerating: messageGenerating,
	})
	if err != nil {
		slog.Error("failed to render index page", "with", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h ChatHandler) deleteChat(w http.ResponseWriter, r *http.Request) {
	// Validate id
	id, err := deserID(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Delete chat
	err = h.q.DeleteChat(r.Context(), id.String())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Redirect
	w.Header().Set("HX-Redirect", h.graphURI)
}

func (h ChatHandler) getEmptyChat(w http.ResponseWriter, r *http.Request) {
	chatTitles, err := findChatsTitles(h.q)
	if err != nil {
		slog.Error("failed to find chat titles", "err", err.Error())
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	err = h.templates.Render(w, "index", ChatViewData{
		Chat: ChatRender{
			ID: uuid.New(),
		},
		Branch:     Branch{ID: uuid.New()},
		ChatTitles: chatTitles,
		Keybinds:   web.Keybinds,
		BaseURI:    h.baseURI,
		GraphURI:   h.graphURI,
	})
	if err != nil {
		slog.Error("failed to render index page", "with", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

type ChatMention struct {
	ID    uuid.UUID
	Title string
}

func (h ChatHandler) postUserMessage(w http.ResponseWriter, r *http.Request) {
	// Validate request
	var errs []error
	prompt := r.FormValue("prompt")
	if prompt == "" {
		errs = append(errs, fmt.Errorf("prompt shouldn't be empty"))
	}

	mentionsJSON := r.FormValue("mentions")
	mentions := []ChatMention{}
	err := json.Unmarshal([]byte(mentionsJSON), &mentions)
	if err != nil {
		errs = append(errs, err)
	}

	id, err := deserID(w, r)
	if err != nil {
		errs = append(errs, err)
	}

	// Branch ID always exists because of routing
	branchID, _, err := deserBranchID(w, r)
	if err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		http.Error(w, errors.Join(errs...).Error(), http.StatusBadRequest)
		return
	}

	// Get chat
	chat, err := findChat(r.Context(), h.q, id)
	userMsg := Message{Text: prompt, Role: "user"}
	var newChatCreated bool
	switch err {
	case nil:
		break
	// TODO: Move to service
	case sql.ErrNoRows:
		// Create new chat
		chat = Chat{
			Title:    "New Chat",
			ID:       id,
			Messages: []Message{userMsg},
		}
		err := saveChat(r.Context(), h.q, chat)
		if err != nil {
			slog.Error("failed to initialize chat", "with", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		newChatCreated = true

		// Generate title in background
		titleCtx := context.Background()
		go func() {
			stream := h.titleChan.Alloc(id)
			defer h.titleChan.Free(id)

			// Generate title
			t, err := genTitle(titleCtx, h.g, prompt)
			if err != nil {
				slog.Error("failed to generate title", "with", err)
				return
			}

			// Publish title
			stream.Chunks <- t

			// Persist title
			err = h.q.SaveChatTitle(titleCtx, db.SaveChatTitleParams{
				ID:    chat.ID.String(),
				Title: t,
			})
			if err != nil {
				slog.Error("failed to save chat title", "with", err)
			}
		}()
	default:
		slog.Error("failed to find chat", "err", err.Error())
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Get branch
	branch, err := findChatBranch(r.Context(), h.q, id, branchID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	branch.Messages = append(branch.Messages, userMsg)

	// Get mentioned chats
	mentionedChats := make([]Chat, len(mentions))
	for i, v := range mentions {
		// Find mentioned chat
		m, err := findChat(r.Context(), h.q, v.ID)
		if err != nil {
			slog.Error("failed to find mentioned chat", "err", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Save used mention
		err = h.q.SaveMention(r.Context(), db.SaveMentionParams{
			TargetID: v.ID.String(),
			SourceID: chat.ID.String(),
		})
		if err != nil {
			slog.Error("failed to save a mention", "with", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		// Collect
		mentionedChats[i] = m
	}

	// Eval prompt
	// TODO: Add timeout
	go func(ctx context.Context, chat Chat) {
		stream := h.msgChan.Alloc(chat.ID)
		defer h.msgChan.Free(chat.ID)

		msg, err := generateMessage(ctx, h.g,
			slices.Concat(chat.Messages, branch.Messages),
			mentionedChats,
			stream.Chunks,
		)
		if err != nil {
			slog.Error("failed to generate message", "with", err)
			return
		}
		branch.Messages = append(branch.Messages, msg)
		err = updateBranchMessages(ctx, h.q, chat.ID, branch)
		if err != nil {
			slog.Error("failed to save chat after generation", "with", err)
		}
	}(context.Background(), chat)

	// Redirect to the new page
	if newChatCreated || len(branch.Messages) == 1 {
		slog.Info("New chat & branch created")
		w.Header().Set(
			"HX-Redirect",
			fmt.Sprintf("%s/%s/branch/%s", h.baseURI, chat.ID.String(), branchID.String()),
		)
		return
	}

	// Render messages
	err = h.templates.Render(w, "message", branch.Messages[len(branch.Messages)-1])
	if err != nil {
		slog.Error("failed to render user message", "with", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	err = h.templates.Render(w, "streamed-message", StreamedMessageView{
		ChatID:   chat.ID.String(),
		BranchID: branch.ID.String(),
		BaseURI:  h.baseURI,
	})
	if err != nil {
		slog.Error("failed to render streamed message", "with", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type StreamedMessageView struct {
	ChatID   string
	BranchID string
	BaseURI  string
}

func (h ChatHandler) getTitle(w http.ResponseWriter, r *http.Request) {
	// Validate data
	id, err := deserID(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Wait for the generated title
	stream, ok := h.titleChan.Get(id)
	if !ok {
		http.Error(w, "There is no any generating title", http.StatusNotFound)
		return
	}

	// Render result
	title := <-stream.Chunks
	_, err = w.Write([]byte(title))
	if err != nil {
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

	stream, ok := h.msgChan.Get(id)
	if !ok {
		sse.Send(w, sse.Event{
			Type: "finished",
			Data: "There is no stream",
		})
		return
	}

	msg := &HTMLMessage{
		Role: "model",
	}

	var raw string

loop:
	for {
		select {
		case chunk := <-stream.Chunks:
			raw += chunk
			msg.Text = markdownToHTML(raw)
			var tpl bytes.Buffer
			if err := h.templates.Render(&tpl, "message", msg); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			sse.Send(w, sse.Event{
				Type: "chunk",
				Data: strings.Replace(tpl.String(), "\n", "", -1),
			})
		case <-stream.Done:
			break loop
		}
	}

	sse.Send(w, sse.Event{
		Type: "finished",
		Data: "",
	})

	<-r.Context().Done()
}

func (h ChatHandler) getMerge(w http.ResponseWriter, r *http.Request) {
	// Validate data
	chatID, err := deserID(w, r)
	var errs []error
	if err != nil {
		errs = append(errs, err)
	}
	// Branch param always exists because of routing
	branchID, _, err := deserBranchID(w, r)
	if err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		http.Error(w, errors.Join(errs...).Error(), http.StatusBadRequest)
		return
	}

	branch, err := findChatBranch(r.Context(), h.q, chatID, branchID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	if len(branch.Messages) < 2 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	err = h.templates.Render(w, "merge-button", nil)
	if err != nil {
		slog.Error("failed to render tempalte", "with", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

type ChatTags struct {
	ID       string
	Tags     []Tag
	Keybinds web.KeybindsTable
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
		ID:       id.String(),
		Tags:     tags,
		Keybinds: web.Keybinds,
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

func deserBranchID(w http.ResponseWriter, r *http.Request) (id uuid.UUID, exists bool, err error) {
	branchID := r.PathValue("branchId")
	if branchID == "" {
		slog.Info("got empty branchId")
		return
	}
	exists = true
	id, err = uuid.Parse(branchID)
	if err != nil {
		slog.Error("failed to parse branch", "id", id, "with", err)
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
