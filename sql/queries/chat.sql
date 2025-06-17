-- name: FindChat :one
SELECT
    title,
    messages
FROM
    chat
WHERE
    id = ?;

-- name: FindChatTitles :many
SELECT
    id,
    title
FROM
    chat;

-- name: SaveChat :exec
INSERT INTO
    chat (id, title, messages)
VALUES
    (?, ?, ?) ON conflict DO
UPDATE
SET
    title = excluded.title,
    messages = excluded.messages,
    updated_at = unixepoch();

-- name: SaveTag :exec
INSERT INTO
    chat_tag (chat_id, name)
VALUES
    (?, ?);

-- name: FindTags :many
SELECT
    name
FROM
    chat_tag
WHERE
    chat_id = ?;

-- name: DeleteTag :exec
DELETE FROM
    chat_tag
WHERE
    chat_id = ?
    AND name = ?;

-- name: SaveChatTitle :exec
UPDATE
    chat
SET
    title = ?,
    updated_at = unixepoch()
WHERE
    id = ?;

-- name: UpdateChatMessages :exec
UPDATE
    chat
SET
    messages = ?,
    updated_at = unixepoch()
WHERE
    id = ?;

-- name: SaveMention :exec
INSERT INTO
    mention(target_id, source_id)
VALUES
    (?, ?) ON CONFLICT(target_id, source_id) DO NOTHING;

-- name: DeleteChat :exec
DELETE FROM
    chat
WHERE
    id = ?;
