CREATE TABLE IF NOT EXISTS chat_branch (
    id TEXT NOT NULL,
    chat_id TEXT NOT NULL,
    messages BLOB NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
    FOREIGN KEY (chat_id) REFERENCES chat(id) ON DELETE CASCADE,
    PRIMARY KEY(id, chat_id)
);
