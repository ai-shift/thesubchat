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

-- name: FindChatBranch :one
SELECT
    messages
FROM
    chat_branch
WHERE
    chat_id = ?
    AND id = ?;

-- name: SaveOrUpdateChatBranchMessages :exec
INSERT INTO
    chat_branch (id, chat_id, messages)
VALUES
    (?, ?, ?) ON conflict (id, chat_id) DO
UPDATE
SET
    id = excluded.id,
    chat_id = excluded.chat_id,
    messages = excluded.messages;

-- name: FindChatBranches :many
SELECT
    id
FROM
    chat_branch
WHERE
    chat_id = ?;

-- name: SaveChatLog :exec
INSERT INTO
    chat_log (chat_id, ACTION, meta)
VALUES
    (?, ?, ?);

-- name: FindChatLog :many
SELECT
    ACTION,
    meta
FROM
    chat_log
WHERE
    chat_id = ?;
