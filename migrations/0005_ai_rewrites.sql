-- +goose Up
-- What the AI proposed for a line or the summary of a CV, kept so that the same text is not paid for twice.
-- input_hash is a digest of what the AI read (section, text, language, offer title). Rows made this month are what
-- the plan's monthly limit of rewrites counts.
CREATE TABLE ai_rewrites (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL,
  resume_id uuid NOT NULL,
  input_hash text NOT NULL CHECK (length(input_hash) = 64),
  suggestions jsonb NOT NULL DEFAULT '[]',
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, input_hash)
);
CREATE INDEX ai_rewrites_user_created_idx ON ai_rewrites (user_id, created_at);

-- +goose Down
DROP TABLE ai_rewrites;
