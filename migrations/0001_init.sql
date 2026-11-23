-- +goose Up
-- Draft schema generated from models/db. Refine (constraints, indexes, FKs) as features land;
-- nothing is deployed yet, so editing this file is fine until the first release.
CREATE TABLE resumes (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL,
  title text NOT NULL DEFAULT '',
  content jsonb NOT NULL DEFAULT '{}',
  template_id uuid,
  version integer NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE templates (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL DEFAULT '',
  layout text NOT NULL DEFAULT '',
  preview_url text NOT NULL DEFAULT '',
  active boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE resume_files (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  resume_id uuid NOT NULL,
  kind text NOT NULL DEFAULT '',
  format text NOT NULL DEFAULT '',
  filename text NOT NULL DEFAULT '',
  content_type text NOT NULL DEFAULT '',
  size_bytes bigint NOT NULL DEFAULT 0,
  sha256 text NOT NULL DEFAULT '',
  data bytea NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE resume_files;
DROP TABLE templates;
DROP TABLE resumes;
