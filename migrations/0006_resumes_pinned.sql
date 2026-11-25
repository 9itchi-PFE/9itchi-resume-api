-- +goose Up
-- pinned_at: when the student chose this CV as the one to keep editing beyond the plan's CV limit. Without a
-- choice, the last modified CVs stay editable.
ALTER TABLE resumes ADD COLUMN pinned_at timestamptz;

-- +goose Down
ALTER TABLE resumes DROP COLUMN pinned_at;
