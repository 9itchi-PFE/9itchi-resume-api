-- +goose Up
-- Three two-column templates: a filled side column (sidebar), a header over two ruled columns (split), an accent band
-- over a grey side column (banner).
ALTER TABLE templates DROP CONSTRAINT templates_layout_check;
ALTER TABLE templates ADD CONSTRAINT templates_layout_check
  CHECK (layout IN ('classic', 'modern', 'compact', 'sidebar', 'split', 'banner'));

INSERT INTO templates (name, layout) VALUES
  ('Sidebar', 'sidebar'),
  ('Split', 'split'),
  ('Banner', 'banner')
ON CONFLICT (name) DO NOTHING;

-- +goose Down
DELETE FROM templates WHERE layout IN ('sidebar', 'split', 'banner');
ALTER TABLE templates DROP CONSTRAINT templates_layout_check;
ALTER TABLE templates ADD CONSTRAINT templates_layout_check CHECK (layout IN ('classic', 'modern', 'compact'));
