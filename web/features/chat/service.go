package chat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"shellshift/internal/db"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/google/uuid"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

type Chat struct {
	ID       uuid.UUID
	Title    string
	Messages []Message
}

type Message struct {
	Text string
	Role string
}

func findChat(ctx context.Context, q *db.Queries, id uuid.UUID) (Chat, error) {
	chat, err := q.FindChat(ctx, id.String())
	if err != nil {
		return Chat{}, err
	}
	var msgs []Message
	err = json.Unmarshal(chat.Messages, &msgs)
	if err != nil {
		return Chat{}, err
	}
	return Chat{
		ID:       id,
		Title:    chat.Title,
		Messages: msgs,
	}, nil
}

type Branch struct {
	ID       uuid.UUID
	Messages []Message
}

func findChatBranch(ctx context.Context, q *db.Queries, chatID uuid.UUID, branchID uuid.UUID) (b Branch, _ error) {
	b.ID = branchID
	msgsBlob, err := q.FindChatBranch(ctx, db.FindChatBranchParams{
		ID:     branchID.String(),
		ChatID: chatID.String(),
	})
	switch err {
	case nil:
		break
	case sql.ErrNoRows:
		return b, nil
	default:
		return b, err
	}
	err = json.Unmarshal(msgsBlob, &b.Messages)
	if err != nil {
		return b, err
	}
	return b, nil
}

func findChatsTitles(q *db.Queries) ([]db.FindChatTitlesRow, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	chats, err := q.FindChatTitles(ctx)
	if err != nil {
		return chats, err
	}
	return chats, nil
}

type generatedTitle struct {
	Title string `json:"title"`
}

func genTitle(ctx context.Context, g *genkit.Genkit, msg string) (string, error) {
	slog.Info("generating title chat")
	prompt := genkit.LookupPrompt(g, "title-generation")
	if prompt == nil {
		return "", fmt.Errorf("failed to find title generation prompt")
	}
	resp, err := prompt.Execute(ctx, ai.WithInput(map[string]any{"query": msg}))
	if err != nil {
		return "", err
	}
	var output generatedTitle
	if err := resp.Output(&output); err != nil {
		return "", fmt.Errorf("failed to parse router output with %w", err)
	}
	return output.Title, err
}

func saveChat(ctx context.Context, q *db.Queries, c Chat) error {
	slog.Info("saving chat", "id", c.ID)
	encoded, err := json.Marshal(c.Messages)
	if err != nil {
		slog.Error("failed to encode messages", "err", err)
		return err
	}
	err = q.SaveChat(ctx, db.SaveChatParams{
		ID:       c.ID.String(),
		Title:    c.Title,
		Messages: encoded,
	})
	if err != nil {
		slog.Error("failed to save chat", "err", err)
		return err
	}
	return nil
}

func updateChatMessages(ctx context.Context, q *db.Queries, c Chat) error {
	slog.Info("updating chat messages", "id", c.ID)
	encoded, err := json.Marshal(c.Messages)
	if err != nil {
		slog.Error("failed to encode messages", "err", err)
		return err
	}
	err = q.UpdateChatMessages(ctx, db.UpdateChatMessagesParams{
		ID:       c.ID.String(),
		Messages: encoded,
	})
	if err != nil {
		slog.Error("failed to update chat messages", "err", err)
		return err
	}
	return nil
}

func updateBranchMessages(ctx context.Context, q *db.Queries, chatID uuid.UUID, b Branch) error {
	slog.Info("updating branch messages", "chatId", chatID, "id", b.ID)
	encoded, err := json.Marshal(b.Messages)
	if err != nil {
		slog.Error("failed to encode branch messages", "err", err)
		return err
	}
	err = q.SaveOrUpdateChatBranchMessages(ctx, db.SaveOrUpdateChatBranchMessagesParams{
		ID:       b.ID.String(),
		ChatID:   chatID.String(),
		Messages: encoded,
	})
	if err != nil {
		slog.Error("failed to persist branch messages", "err", err)
		return err
	}
	return nil
}

func generateMessage(ctx context.Context, g *genkit.Genkit, msgs []Message, mentioned []Chat, s chan<- string) (msg Message, err error) {
	slog.Info("Starting message generation")
	// Prepare messages
	mapped := make([]*ai.Message, len(msgs))
	for i, msg := range msgs {
		mapped[i] = ai.NewTextMessage(ai.Role(msg.Role), msg.Text)
	}

	docs := make([]*ai.Document, len(mentioned))
	for i, c := range mentioned {
		b, err := json.Marshal(c)
		if err != nil {
			return msg, fmt.Errorf("faield to serialize mentioned chats with %w", err)
		}
		docs[i] = ai.DocumentFromText(string(b), map[string]any{
			"chatTitle": c.Title,
		})
	}

	// Request model
	resp, err := genkit.Generate(ctx, g,
		ai.WithMessages(mapped...),
		ai.WithDocs(docs...),
		ai.WithStreaming(func(ctx context.Context, chunk *ai.ModelResponseChunk) error {
			s <- chunk.Text()
			return nil
		}),
	)
	if err != nil {
		return
	}
	slog.Info("model response", "length", len(resp.Text()))
	msg.Role = "model"
	msg.Text = resp.Text()
	return
}

type HTMLMessage struct {
	Role string
	Text template.HTML
}

func renderMessages(chat Chat) []HTMLMessage {
	htmlMessages := make([]HTMLMessage, len(chat.Messages))

	for i, v := range chat.Messages {
		htmlMessages[i] = HTMLMessage{
			Role: v.Role,
			Text: markdownToHTML(v.Text),
		}
	}

	return htmlMessages
}

func renderMessage(msg Message) HTMLMessage {
	return HTMLMessage{
		Role: msg.Role,
		Text: markdownToHTML(msg.Text),
	}
}

func markdownToHTML(markdownStr string) template.HTML {
	extentions := parser.CommonExtensions | parser.MathJax | parser.SuperSubscript | parser.AutoHeadingIDs

	p := parser.NewWithExtensions(extentions)
	doc := p.Parse([]byte(markdownStr))

	htmlFlags := html.CommonFlags
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)

	return template.HTML(string(markdown.Render(doc, renderer)))
}
