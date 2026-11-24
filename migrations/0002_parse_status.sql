-- +goose Up
-- Sprint 2: an uploaded CV is read by ai-api in the background. parse_status drives the "reading your CV" screen:
-- none (never parsed), pending, done, failed. language is the CV's language (fr, en, ar) once read.
ALTER TABLE resumes
  ADD COLUMN parse_status text NOT NULL DEFAULT 'none' CHECK (parse_status IN ('none', 'pending', 'done', 'failed')),
  ADD COLUMN language text CHECK (language IS NULL OR language IN ('fr', 'en', 'ar'));

-- One row per CV download, kept when the CV is deleted so the free download cannot be reset by deleting a CV.
CREATE TABLE resume_downloads (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL,
  resume_id uuid,
  format text NOT NULL CHECK (format IN ('pdf', 'docx')),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX resume_downloads_user_idx ON resume_downloads (user_id);

-- +goose Down
DROP TABLE resume_downloads;
ALTER TABLE resumes
  DROP COLUMN language,
  DROP COLUMN parse_status;
