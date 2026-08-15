CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX chat_messages_message_text_trgm_idx
    ON chat.chat_messages USING GIN (message_text gin_trgm_ops);
