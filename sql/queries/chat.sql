-- name: FindChat :one
SELECT
    title,
    messages
FROM
    chat
WHERE
    id = ?;

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
