package db

import "time"

// Resume maps table resumes.
type Resume struct {
	ID         string         `db:"id" json:"id"`
	UserID     string         `db:"user_id" json:"user_id"`
	Title      string         `db:"title" json:"title"`
	Content    map[string]any `db:"content" json:"content"`
	TemplateID *string        `db:"template_id" json:"template_id"`
	Version    int            `db:"version" json:"version"`
	CreatedAt  time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time      `db:"updated_at" json:"updated_at"`
}
