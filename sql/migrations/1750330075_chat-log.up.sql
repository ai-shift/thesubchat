CREATE TABLE chat_log (
    chat_id text NOT NULL,
    ACTION text NOT NULL,
    meta blob,
    FOREIGN KEY (chat_id) REFERENCES chat(id) ON DELETE CASCADE
);
