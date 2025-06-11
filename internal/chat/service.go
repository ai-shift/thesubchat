package chat

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"shellshift/internal/chat/llm"
	"shellshift/internal/db"
)

type Chat struct {
	ID       uuid.UUID
	Title    string
	Messages []llm.Message
}

func findChat(q *db.Queries, id uuid.UUID) (*Chat, error) {
	ctx := context.Background()
	chat, err := q.FindChat(ctx, id.String())
	if err != nil {
		return nil, err
	}
	var msgs []llm.Message
	err = json.Unmarshal(chat.Messages, &msgs)
	if err != nil {
		return nil, err
	}
	return &Chat{
		ID:       id,
		Title:    chat.Title.String,
		Messages: msgs,
	}, nil
}
