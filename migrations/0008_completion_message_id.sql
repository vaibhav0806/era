-- +goose Up
ALTER TABLE tasks ADD COLUMN completion_message_id INTEGER;

-- +goose Down
SELECT 1;
