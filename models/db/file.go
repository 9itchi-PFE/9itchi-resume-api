package db

import "time"

// ResumeFile maps table resume_files.
type ResumeFile struct {
	ID          string    `db:"id" json:"id"`
	ResumeID    string    `db:"resume_id" json:"resume_id"`
	Kind        string    `db:"kind" json:"kind"`
	Format      string    `db:"format" json:"format"`
	Filename    string    `db:"filename" json:"filename"`
	ContentType string    `db:"content_type" json:"content_type"`
	SizeBytes   int64     `db:"size_bytes" json:"size_bytes"`
	SHA256      string    `db:"sha256" json:"sha256"`
	Data        []byte    `db:"data" json:"-"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// ResumeFileMeta is a resume_files row without the data column (list and metadata queries).
type ResumeFileMeta struct {
	ID          string    `db:"id"`
	ResumeID    string    `db:"resume_id"`
	Kind        string    `db:"kind"`
	Format      string    `db:"format"`
	Filename    string    `db:"filename"`
	ContentType string    `db:"content_type"`
	SizeBytes   int64     `db:"size_bytes"`
	SHA256      string    `db:"sha256"`
	CreatedAt   time.Time `db:"created_at"`
}
