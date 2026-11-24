-- +goose Up
-- When a student was last given the weekly advice on their CV, and for which CV. One row per student: a claim
-- writes it first, so several instances never tip the same student twice.
CREATE TABLE cv_tips (
  user_id uuid PRIMARY KEY,
  resume_id uuid NOT NULL,
  sent_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE cv_tips;
