CREATE TABLE default_llm (model TEXT, config BLOB, messages BLOB);

CREATE TABLE chat (
    id INTEGER PRIMARY KEY,
    title TEXT,
    messages BLOB,
    vcs BLOB,
    model TEXT
);

CREATE TABLE chat_mention (
    src_chat_id INTEGER,
    dest_chat_id INTEGER,
    PRIMARY KEY (src_chat_id, dest_chat_id),
    FOREIGN KEY (src_chat_id) REFERENCES chat (id),
    FOREIGN KEY (dest_chat_id) REFERENCES chat (id)
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
