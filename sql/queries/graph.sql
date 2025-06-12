-- name: GetGraph :many
SELECT
    c.id,
    c.title,
    c.updated_at,
    t.name
FROM
    chat_tag t
    RIGHT JOIN chat c ON t.chat_id = c.id;
