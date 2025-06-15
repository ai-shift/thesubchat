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
	slog.Info("saving cchat", "id", c.ID)
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
		slog.Error("failed to save c", "err", err)
		return err
	}
	return nil

}

func generateMessage(ctx context.Context, g *genkit.Genkit, msgs []Message, mentioned []Chat, s chan<- string) (msg Message, err error) {
	slog.Info("Starting message generation")
	mapped := make([]*ai.Message, len(msgs))
	for i, msg := range msgs {
		mapped[i] = ai.NewTextMessage(ai.Role(msg.Role), msg.Text)
	}
  mentionedBlob, err := json.Marshal(mentioned)
  if err != nil {
    return
  }
  mapped = append(mapped, &ai.Message{
    Content: []*ai.Part{ ai.NewJSONPart(string(mentionedBlob))},
  },
  )
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
	msg.Role = "assistant"
	msg.Text = resp.Text()
	return
}
