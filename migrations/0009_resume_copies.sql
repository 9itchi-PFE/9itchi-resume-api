-- +goose Up
-- A copy of a CV (POST /resumes/:id/duplicate) points to the CV it was made from (the root, never another copy): a
-- CV and its copies count once against the plan's "resumes" limit, since duplicating is not importing.
ALTER TABLE resumes ADD COLUMN source_id uuid;

-- +goose Down
ALTER TABLE resumes DROP COLUMN source_id;
