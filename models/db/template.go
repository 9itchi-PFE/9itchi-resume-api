package db

import "time"

// Template maps table templates.
// TemplateUsage is a template with how many CVs use it.
type TemplateUsage struct {
	Template
	Resumes int `db:"resumes" json:"resumes"`
}

type Template struct {
	ID         string    `db:"id" json:"id"`
	Name       string    `db:"name" json:"name"`
	Layout     string    `db:"layout" json:"layout"`
	PreviewURL string    `db:"preview_url" json:"preview_url"`
	Active     bool      `db:"active" json:"active"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at" json:"updated_at"`
}
