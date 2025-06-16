package chat

import (
	"context"
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

func findChat(q *db.Queries, id uuid.UUID) (Chat, error) {
	ctx := context.Background()
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

func findChatsTitles(q *db.Queries) ([]db.FindChatTitlesRow, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	chats, err := q.FindChatTitles(ctx)
	if err != nil {
		return nil, err
	}
	return chats, nil
}

func genTitle(ctx context.Context, g *genkit.Genkit, msg string) (string, error) {
	slog.Info("generating title chat")
	resp, err := genkit.Generate(ctx, g,
		ai.WithModelName("googleai/gemini-2.0-flash"),
		ai.WithPrompt("Create a concise, 3-5 word phrase as a header for the following query, strictly adhering to the 3-5 word limit and avoiding the use of the word 'title':", msg),
	)
	if err != nil {
		return "", err
	}
	return resp.Text(), err
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

func generateMessage(ctx context.Context, g *genkit.Genkit, msgs []Message, mentioned []Chat, s chan<- string) (msg Message, err error) {
	slog.Info("Starting message generation")
	// Prepare messages
	mapped := make([]*ai.Message, len(msgs))
	for i, msg := range msgs {
		mapped[i] = ai.NewTextMessage(ai.Role(msg.Role), msg.Text)
	}
	mentionedBlob, err := json.Marshal(mentioned)
	if err != nil {
		return msg, fmt.Errorf("faield to serialize mentioned chats with %w", err)
	}
	slog.Info("blob", "text", string(mentionedBlob))
	mapped = append(mapped, ai.NewTextMessage(ai.RoleUser, string(mentionedBlob)))

	for _, msg := range mapped {
		slog.Info("Got message", "msg", fmt.Sprintf("%#v", msg))
	}

	// Request model
	resp, err := genkit.Generate(ctx, g,
		ai.WithMessages(mapped...),
		ai.WithStreaming(func(ctx context.Context, chunk *ai.ModelResponseChunk) error {
			s <- chunk.Text()
			return nil
		}),
	)
	if err != nil {
		return
	}
	slog.Info("model response", "text", resp.Text())
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

func markdownToHTML(markdownStr string) template.HTML {
	extentions := parser.CommonExtensions | parser.MathJax | parser.SuperSubscript | parser.AutoHeadingIDs

	p := parser.NewWithExtensions(extentions)
	doc := p.Parse([]byte(markdownStr))

	htmlFlags := html.CommonFlags
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)

	return template.HTML(string(markdown.Render(doc, renderer)))
}
