// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.29.0
// source: graph.sql

package db

import (
	"context"
	"database/sql"
)

const findChatMentions = `-- name: FindChatMentions :many
SELECT
    target_id,
    source_id
FROM
    mention
`

type FindChatMentionsRow struct {
	TargetID string
	SourceID string
}

func (q *Queries) FindChatMentions(ctx context.Context) ([]FindChatMentionsRow, error) {
	rows, err := q.db.QueryContext(ctx, findChatMentions)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []FindChatMentionsRow
	for rows.Next() {
		var i FindChatMentionsRow
		if err := rows.Scan(&i.TargetID, &i.SourceID); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const findChatTags = `-- name: FindChatTags :many
SELECT
    c.id,
    c.title,
    c.updated_at,
    t.name
FROM
    chat_tag t
    RIGHT JOIN chat c ON t.chat_id = c.id
`

type FindChatTagsRow struct {
	ID        string
	Title     string
	UpdatedAt int64
	Name      sql.NullString
}

func (q *Queries) FindChatTags(ctx context.Context) ([]FindChatTagsRow, error) {
	rows, err := q.db.QueryContext(ctx, findChatTags)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []FindChatTagsRow
	for rows.Next() {
		var i FindChatTagsRow
		if err := rows.Scan(
			&i.ID,
			&i.Title,
			&i.UpdatedAt,
			&i.Name,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
