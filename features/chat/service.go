package chat

import (
	"context"
	"encoding/json"
	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/google/uuid"
	"log/slog"
	"shellshift/internal/db"
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

func findChat(q *db.Queries, id uuid.UUID) (*Chat, error) {
	ctx := context.Background()
	chat, err := q.FindChat(ctx, id.String())
	if err != nil {
		return nil, err
	}
	var msgs []Message
	err = json.Unmarshal(chat.Messages, &msgs)
	if err != nil {
		return nil, err
	}
	return &Chat{
		ID:       id,
		Title:    chat.Title,
		Messages: msgs,
	}, nil
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

func generateMessage(ctx context.Context, g *genkit.Genkit, msgs []Message) ([]Message, error) {
	mapped := make([]*ai.Message, len(msgs))
	for i, msg := range msgs {
		mapped[i] = ai.NewTextMessage(ai.Role(msg.Role), msg.Text)
	}
	resp, err := genkit.Generate(ctx, g, ai.WithMessages(mapped...))
	if err != nil {
		return msgs, err
	}
	msg := Message{
		Text: resp.Text(),
		Role: "assistant",
	}
	msgs = append(msgs, msg)
	return msgs, nil
}
