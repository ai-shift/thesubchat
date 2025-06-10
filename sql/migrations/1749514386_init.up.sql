CREATE TABLE default_llm (model TEXT, config BLOB, messages BLOB);

CREATE TABLE chat (
    id INTEGER PRIMARY KEY,
    title TEXT,
    messages BLOB,
    vcs BLOB,
    model TEXT
);

CREATE TABLE chat_mention (
    from_chat_id INTEGER,
    from_chat_commit TEXT,
    to_chat_id INTEGER,
    to_chat_commit TEXT,
    PRIMARY KEY (from_chat_id, from_chat_commit, to_chat_id, to_chat_commit),
    FOREIGN KEY (from_chat_id) REFERENCES chat (id),
    FOREIGN KEY (to_chat_id) REFERENCES chat (id)
);

CREATE TABLE chat_tag (
    id INTEGER,
    name TEXT,
    PRIMARY KEY (id)
);

CREATE TABLE chat2tag (
    chat_id INTEGER,
    tag_id INTEGER,
    PRIMARY KEY (chat_id, tag_id),
    FOREIGN KEY (chat_id) REFERENCES chat (id),
    FOREIGN KEY (tag_id) REFERENCES chat_tag (id)
);
