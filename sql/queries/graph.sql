-- name: GetGraph :many
SELECT
    id,
    title
FROM
    chat
ORDER BY
    updated_at DESC;
