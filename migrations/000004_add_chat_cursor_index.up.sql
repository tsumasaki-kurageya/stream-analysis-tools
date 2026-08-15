CREATE INDEX chat_messages_stream_offset_id_idx
    ON chat.chat_messages (stream_id, offset_milliseconds, id);
