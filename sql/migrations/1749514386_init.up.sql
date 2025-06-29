CREATE TABLE IF NOT EXISTS chat (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    messages BLOB NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE IF NOT EXISTS chat_tag (
    chat_id text NOT NULL,
    name text NOT NULL,
    PRIMARY KEY (chat_id, name),
    FOREIGN KEY (chat_id) REFERENCES chat (id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS mention (
    source_id TEXT NOT NULL,
    target_id TEXT NOT NULL CHECK (target_id != source_id),
    PRIMARY KEY (source_id, target_id),
    FOREIGN KEY (source_id) REFERENCES chat (id) ON DELETE CASCADE,
    FOREIGN KEY (target_id) REFERENCES chat (id) ON DELETE CASCADE
)
