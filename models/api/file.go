package api

import "time"

// FileResponse is file metadata, never the blob.
type FileResponse struct {
	ID          string    `json:"id"`
	ResumeID    string    `json:"resume_id"`
	Kind        string    `json:"kind"`
	Format      string    `json:"format"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	SHA256      string    `json:"sha256"`
	CreatedAt   time.Time `json:"created_at"`
}

type ListFilesResponse struct {
	Items []FileResponse `json:"items"`
}

// FileContent is a blob ready to stream. Filename is already sanitized.
type FileContent struct {
	Filename    string
	ContentType string
	Data        []byte
}
