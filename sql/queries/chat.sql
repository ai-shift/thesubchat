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
    (?, ?, ?);

-- name: UpdateChat :exec
UPDATE
    chat
SET
    messages = ?
WHERE
    id = ?;
