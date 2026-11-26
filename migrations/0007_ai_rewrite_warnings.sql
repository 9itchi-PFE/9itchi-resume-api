-- +goose Up
-- Codes ai-api raised on the proposal (new_numbers, too_long), kept with it so a cached answer shows them again.
ALTER TABLE ai_rewrites ADD COLUMN warnings jsonb NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE ai_rewrites DROP COLUMN warnings;
