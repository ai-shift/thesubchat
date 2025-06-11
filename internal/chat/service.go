package chat

import (
	"context"
	"encoding/json"
	"shellshift/internal/chat/llm"
	"shellshift/internal/db"
)

type Chat struct {
	ID       int64
	Title    string
	Messages []llm.Message
}

func findChat(q *db.Queries, id int64) (*Chat, error) {
	ctx := context.Background()
	chat, err := q.FindChat(ctx, id)
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
		Title:    chat.Title,
		Messages: msgs,
	}, nil
}
