package core

import (
	"context"

	"9itchi-resume-api/models/api"
)

// Exporter renders a resume to a file format (Strategy). layout is the template layout name
// (classic, modern, compact); unknown layouts fall back to classic.
type Exporter interface {
	Format() string
	Render(ctx context.Context, content api.ResumeContent, layout string) ([]byte, error)
}

// DefaultExporters is the registry keyed by format. Only pdf is registered; docx is unsupported.
func DefaultExporters() map[string]Exporter {
	return map[string]Exporter{
		"pdf": NewPDFExporter(),
	}
}
