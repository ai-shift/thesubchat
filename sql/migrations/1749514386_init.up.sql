CREATE TABLE IF NOT EXISTS chat (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    messages BLOB NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE chat_tag (
    chat_id text,
    name text NOT NULL UNIQUE,
    FOREIGN KEY (chat_id) REFERENCES chat (id)
);
