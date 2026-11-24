-- +goose Up
-- The CV photo (optional on the Classic and Modern templates) is a resume file of kind photo (JPEG or PNG).
ALTER TABLE resume_files DROP CONSTRAINT resume_files_kind_check;
ALTER TABLE resume_files ADD CONSTRAINT resume_files_kind_check CHECK (kind IN ('original', 'export', 'photo'));
ALTER TABLE resume_files DROP CONSTRAINT resume_files_format_check;
ALTER TABLE resume_files ADD CONSTRAINT resume_files_format_check CHECK (format IN ('pdf', 'docx', 'jpeg', 'png'));

-- What the AI changed while reading an uploaded CV ("Ce qu'on a changé", screen 1.2c).
ALTER TABLE resumes ADD COLUMN parse_report jsonb CHECK (parse_report IS NULL OR jsonb_typeof(parse_report) = 'object');

-- A short key saying what a saved version was before (history tab of screen 4.1): bullet_rewritten,
-- template_changed, section_added, before_restore, manual... The client translates it.
ALTER TABLE resume_versions ADD COLUMN label text NOT NULL DEFAULT '' CHECK (length(label) <= 100);

-- +goose Down
ALTER TABLE resume_versions DROP COLUMN label;
ALTER TABLE resumes DROP COLUMN parse_report;
DELETE FROM resume_files WHERE kind = 'photo';
ALTER TABLE resume_files DROP CONSTRAINT resume_files_format_check;
ALTER TABLE resume_files ADD CONSTRAINT resume_files_format_check CHECK (format IN ('pdf', 'docx'));
ALTER TABLE resume_files DROP CONSTRAINT resume_files_kind_check;
ALTER TABLE resume_files ADD CONSTRAINT resume_files_kind_check CHECK (kind IN ('original', 'export'));
