-- +goose Up
-- Sprint 2: an uploaded CV is read by ai-api in the background. parse_status drives the "reading your CV" screen:
-- none (never parsed), pending, done, failed. language is the CV's language (fr, en, ar) once read.
ALTER TABLE resumes
  ADD COLUMN parse_status text NOT NULL DEFAULT 'none' CHECK (parse_status IN ('none', 'pending', 'done', 'failed')),
  ADD COLUMN language text CHECK (language IS NULL OR language IN ('fr', 'en', 'ar'));

-- +goose Down
ALTER TABLE resumes
  DROP COLUMN language,
  DROP COLUMN parse_status;
