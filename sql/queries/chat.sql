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
    upated_at = unixepoch();
