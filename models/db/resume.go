package db

import "time"

// Resume maps table resumes. Content is the raw jsonb document (api.ResumeContent).
type Resume struct {
	ID         string  `db:"id" json:"id"`
	UserID     string  `db:"user_id" json:"user_id"`
	Title      string  `db:"title" json:"title"`
	Content    []byte  `db:"content" json:"-"`
	TemplateID *string `db:"template_id" json:"template_id"`
	Version    int     `db:"version" json:"version"`
	// ParseStatus is none|pending|done|failed (AI reading of an uploaded CV); Language is fr|en|ar once known.
	ParseStatus string  `db:"parse_status" json:"parse_status"`
	Language    *string `db:"language" json:"language"`
	// ParseReport is what the AI changed while reading the upload (jsonb, nil when never read).
	ParseReport []byte    `db:"parse_report" json:"-"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// Parse statuses of resumes.parse_status.
const (
	ParseNone    = "none"
	ParsePending = "pending"
	ParseDone    = "done"
	ParseFailed  = "failed"
)

// ResumeSummary is a resumes row without the content column (list queries).
type ResumeSummary struct {
	ID         string    `db:"id"`
	Title      string    `db:"title"`
	TemplateID *string   `db:"template_id"`
	Version    int       `db:"version"`
	CreatedAt  time.Time `db:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"`
}

// ResumeVersionMeta is a resume_versions row without the content column.
type ResumeVersionMeta struct {
	ID        string    `db:"id"`
	ResumeID  string    `db:"resume_id"`
	VersionNo int       `db:"version_no"`
	Label     string    `db:"label"`
	CreatedAt time.Time `db:"created_at"`
}

// ResumeVersion is a resume_versions row with its content.
type ResumeVersion struct {
	ID        string    `db:"id"`
	ResumeID  string    `db:"resume_id"`
	VersionNo int       `db:"version_no"`
	Label     string    `db:"label"`
	Content   []byte    `db:"content"`
	CreatedAt time.Time `db:"created_at"`
}
