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
	"slices"
	"strings"

	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"github.com/google/uuid"

	"shellshift/internal/db"
	"shellshift/internal/sse"
	"shellshift/internal/templates"
	"shellshift/web"
	"shellshift/web/features/auth"
	"shellshift/web/features/chat/textchan"
)

type ChatHandler struct {
	templates *templates.Templates
	g         *genkit.Genkit
	msgChan   *textchan.TextChan
	titleChan *textchan.TextChan
	baseURI   string
	graphURI  string
	db        *db.Factory
}

func InitMux(dbF *db.Factory, protector *auth.ProtectionMiddleware, baseURI, graphURI string) *http.ServeMux {
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
		g:         g,
		msgChan:   textchan.New(),
		titleChan: textchan.New(),
		baseURI:   baseURI,
		graphURI:  graphURI,
		db:        dbF,
	}
	m := http.NewServeMux()
	m.HandleFunc("GET /", protector.Protect(h.getEmptyChat))
	m.HandleFunc("GET /redirect", protector.Protect(h.redirect))
	m.HandleFunc("GET /{id}", protector.Protect(h.getChat))
	m.HandleFunc("DELETE /{id}", protector.Protect(h.deleteChat))
	m.HandleFunc("GET /{id}/branch", protector.Protect(h.getBranches))
	m.HandleFunc("GET /{id}/branch/{branchId}", protector.Protect(h.getChat))
	m.HandleFunc("POST /{id}/branch/{branchId}/message", protector.Protect(h.postUserMessage))
	m.HandleFunc("GET /{id}/branch/{branchId}/message/stream", protector.Protect(h.getMessageStream))
	m.HandleFunc("GET /{id}/branch/{branchId}/merge-status", protector.Protect(h.getMergeStatus))
	m.HandleFunc("GET /{id}/branch/{branchId}/merge", protector.Protect(h.getMerge))
	m.HandleFunc("POST /{id}/branch/{branchId}/merge", protector.Protect(h.postMerge))
	m.HandleFunc("GET /{id}/title", protector.Protect(h.getTitle))
	m.HandleFunc("GET /{id}/tags", protector.Protect(h.getTags))
	m.HandleFunc("POST /{id}/tags", protector.Protect(h.postTags))
	m.HandleFunc("DELETE /{id}/tags", protector.Protect(h.deleteTags))
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
	ChatTitles        []db.FindChatTitlesRow
	Keybinds          web.KeybindsTable
	BaseURI           string
	GraphURI          string
	MessageGenerating bool
	Empty             bool
}

func (h ChatHandler) redirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, fmt.Sprintf("%s/", h.baseURI), http.StatusMovedPermanently)
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

	q, err := h.getQueries(w, r)
	if err != nil {
		return
	}

	var branch Branch
	if exists {
		branch, err = findChatBranch(r.Context(), q, id, branchID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	} else {
		branch = Branch{ID: uuid.New()}
	}
	chat, err := findChat(r.Context(), q, id)
	if err != nil {
		slog.Error("failed to find chat", "err", err.Error())
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	chatTitles, err := findChatsTitles(q)

	if err != nil {
		slog.Error("failed to find chat titles", "err", err.Error())
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	_, messageGenerating := h.msgChan.Get(branch.ID)
	err = h.templates.Render(w, "index", ChatViewData{
		Chat: ChatRender{
			ID:       chat.ID,
			Title:    chat.Title,
			Messages: renderMessages(chat),
		},
		Branch:            branch,
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

	q, err := h.getQueries(w, r)
	if err != nil {
		return
	}

	// Delete chat
	err = q.DeleteChat(r.Context(), id.String())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Redirect
	w.Header().Set("HX-Redirect", h.graphURI)
}

func (h ChatHandler) getBranches(w http.ResponseWriter, r *http.Request) {
	// Validate data
	chatID, err := deserID(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	q, err := h.getQueries(w, r)
	if err != nil {
		return
	}

	log, err := findChatLog(r.Context(), q, chatID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	slog.Info("found chat log", "length", len(log))

	items := make([]branchTreeViewItem, len(log))
	for i, l := range log {
		items[i] = branchTreeViewItem{
			Meta:   l.Meta,
			Action: l.Action,
		}
	}

	chat, err := findChat(r.Context(), q, chatID)
	if err != nil {
		slog.Error("failed to find chat", "err", err.Error())
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	_, titleGenerating := h.titleChan.Get(chatID)

	// Render response
	err = h.templates.Render(w, "branch-tree", branchTreeView{
		Items: items,
		Chat: ChatRender{
			ID:    chat.ID,
			Title: chat.Title,
		},
		TitleGenerating: titleGenerating,
		BaseURI:         h.baseURI,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

type branchTreeView struct {
	Items           []branchTreeViewItem
	TitleGenerating bool
	Chat            ChatRender
	BaseURI         string
}

type branchTreeViewItem struct {
	Meta   ChatLogger
	Action string
}

func (h ChatHandler) getEmptyChat(w http.ResponseWriter, r *http.Request) {
	q, err := h.getQueries(w, r)
	if err != nil {
		return
	}

	chatTitles, err := findChatsTitles(q)
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
		Empty:      true,
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

	q, err := h.getQueries(w, r)
	if err != nil {
		return
	}

	// Get chat
	chat, err := findChat(r.Context(), q, id)
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
			Messages: []Message{},
		}
		err := saveChat(r.Context(), q, chat)
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
			err = q.SaveChatTitle(titleCtx, db.SaveChatTitleParams{
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
	branch, err := findChatBranch(r.Context(), q, id, branchID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	branch.Messages = append(branch.Messages, userMsg)

	// New branch should be created
	if len(branch.Messages) == 1 {
		err = updateBranchMessages(r.Context(), q, chat.ID, branch)
		if err != nil {
			slog.Error("failed to save new branch", "with", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = saveChatLog(r.Context(), q, chat.ID, LogBranchCreated{
			BranchID:         branch.ID.String(),
			OriginMessageIdx: len(chat.Messages) - 1,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Get mentioned chats
	mentionedChats := make([]Chat, len(mentions))
	for i, v := range mentions {
		// Find mentioned chat
		m, err := findChat(r.Context(), q, v.ID)
		if err != nil {
			slog.Error("failed to find mentioned chat", "err", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Save used mention
		err = q.SaveMention(r.Context(), db.SaveMentionParams{
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
	go func(branch Branch) {
		ctx := context.Background()
		stream := h.msgChan.Alloc(branch.ID)
		defer h.msgChan.Free(branch.ID)

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
		err = updateBranchMessages(ctx, q, chat.ID, branch)
		if err != nil {
			slog.Error("failed to save chat after generation", "with", err)
		}
	}(branch)

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
	view := StreamedMessageView{BaseURI: h.baseURI}
	view.Chat.ID = chat.ID.String()
	view.Branch.ID = branch.ID.String()

	if err := h.templates.Render(w, "streamed-message", view); err != nil {
		slog.Error("failed to render streamed message", "with", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type StreamedMessageView struct {
	Chat struct {
		ID string
	}
	Branch struct {
		ID string
	}
	BaseURI string
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

	branchID, _, err := deserBranchID(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	stream, ok := h.msgChan.Get(branchID)
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

func (h ChatHandler) getMergeStatus(w http.ResponseWriter, r *http.Request) {
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

	q, err := h.getQueries(w, r)
	if err != nil {
		return
	}

	branch, err := findChatBranch(r.Context(), q, chatID, branchID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	if len(branch.Messages) < 2 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	err = h.templates.Render(w, "merge-button", mergeButtonView{BranchID: branch.ID.String()})
	if err != nil {
		slog.Error("failed to render tempalte", "with", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

type mergeButtonView struct {
	BranchID string
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

	q, err := h.getQueries(w, r)
	if err != nil {
		return
	}

	// Find branch
	branch, err := findChatBranch(r.Context(), q, chatID, branchID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	// TODO: Decompose into separate function (look `getMergeStatus`)
	if len(branch.Messages) < 2 {
		http.Error(w, "Branch should countain at least 2 messages", http.StatusBadRequest)
		return
	}

	// Build merge items
	items := make([]mergeViewItem, len(branch.Messages))
	for i, msg := range branch.Messages {
		items[i] = mergeViewItem{
			ID:       i,
			Message:  renderMessage(msg),
			Selected: i == 0 || i == len(branch.Messages)-1,
		}
	}

	// Render tempalte
	if err := h.templates.Render(w, "merge", items); err != nil {
		slog.Error("failed to render tempalte", "with", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

type mergeViewItem struct {
	ID       int
	Message  HTMLMessage
	Selected bool
}

func (h ChatHandler) postMerge(w http.ResponseWriter, r *http.Request) {
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

	q, err := h.getQueries(w, r)
	if err != nil {
		return
	}

	// Find branch
	branch, err := findChatBranch(r.Context(), q, chatID, branchID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	var toMerge []Message
	for idx, msg := range branch.Messages {
		selected := r.FormValue(fmt.Sprintf("merge-item-%d", idx))
		if selected == "on" {
			toMerge = append(toMerge, msg)
		}
	}

	slog.Info("messages to be merged", "length", len(toMerge))
	if len(toMerge) == 0 {
		http.Error(w, "At least one message should be merged", http.StatusBadRequest)
		return
	}

	chat, err := findChat(r.Context(), q, chatID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = saveChatLog(r.Context(), q, chatID, LogBranchMerged{
		BranchID:           branch.ID.String(),
		MergedAmount:       len(toMerge),
		MergedAtMessageIdX: len(chat.Messages) - 1,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	chat.Messages = slices.Concat(chat.Messages, toMerge)
	err = updateChatMessages(r.Context(), q, chat)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	w.Header().Set("HX-Redirect", fmt.Sprintf("%s/%s", h.baseURI, chatID))
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

	q, err := h.getQueries(w, r)
	if err != nil {
		return
	}

	// Persist tag
	err = q.SaveTag(r.Context(), db.SaveTagParams{
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

	q, err := h.getQueries(w, r)
	if err != nil {
		return
	}

	rows, err := q.FindTags(r.Context(), id.String())
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

	q, err := h.getQueries(w, r)
	if err != nil {
		return
	}

	// Delete tag
	err = q.DeleteTag(r.Context(), db.DeleteTagParams{
		ChatID: id.String(),
		Name:   tag,
	})
	if err != nil {
		slog.Error("failed to delete tag", "tag", tag, "with", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h ChatHandler) getQueries(w http.ResponseWriter, r *http.Request) (*db.Queries, error) {
	userID := r.Context().Value(auth.UserIDKey)
	q, err := h.db.Get(userID.(string))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return q, err
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
