-- +goose Up
-- Nothing is deployed yet, so editing this file is fine until the first release.
CREATE TABLE templates (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL UNIQUE CHECK (char_length(name) BETWEEN 1 AND 100),
  layout text NOT NULL CHECK (layout IN ('classic', 'modern', 'compact')),
  preview_url text NOT NULL DEFAULT '' CHECK (char_length(preview_url) <= 500),
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO templates (name, layout) VALUES
  ('Classic', 'classic'),
  ('Modern', 'modern'),
  ('Compact', 'compact');

CREATE TABLE resumes (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL,
  title text NOT NULL DEFAULT '' CHECK (char_length(title) <= 200),
  content jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(content) = 'object'),
  template_id uuid REFERENCES templates (id) ON DELETE SET NULL,
  version integer NOT NULL DEFAULT 0 CHECK (version >= 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX resumes_user_updated_idx ON resumes (user_id, updated_at DESC);
CREATE INDEX resumes_template_idx ON resumes (template_id);

CREATE TABLE resume_versions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  resume_id uuid NOT NULL REFERENCES resumes (id) ON DELETE CASCADE,
  version_no integer NOT NULL CHECK (version_no > 0),
  content jsonb NOT NULL CHECK (jsonb_typeof(content) = 'object'),
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (resume_id, version_no)
);

CREATE TABLE resume_files (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  resume_id uuid NOT NULL REFERENCES resumes (id) ON DELETE CASCADE,
  kind text NOT NULL CHECK (kind IN ('original', 'export')),
  format text NOT NULL CHECK (format IN ('pdf', 'docx')),
  filename text NOT NULL DEFAULT '',
  content_type text NOT NULL DEFAULT '',
  size_bytes bigint NOT NULL CHECK (size_bytes > 0 AND size_bytes <= 10485760),
  sha256 text NOT NULL CHECK (char_length(sha256) = 64),
  data bytea NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX resume_files_resume_kind_idx ON resume_files (resume_id, kind, created_at DESC);

-- +goose Down
DROP TABLE resume_files;
DROP TABLE resume_versions;
DROP TABLE resumes;
DROP TABLE templates;
