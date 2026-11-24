package core

import (
	"context"

	"9itchi-resume-api/models/api"
)

// Exporter renders a resume to a file format (Strategy). layout is the template layout name
// (classic, modern, compact); unknown layouts fall back to classic.
type Exporter interface {
	Format() string
	Render(ctx context.Context, content api.ResumeContent, layout string, opts RenderOptions) ([]byte, error)
}

// RenderOptions tune an export.
type RenderOptions struct {
	// Language of the section headings: "fr" (default) or "en". Arabic CVs use French headings for now
	// (the core PDF fonts have no Arabic glyphs).
	Language string
	// Branded adds the "made with 9itchi" mark to every page (free plan download).
	Branded bool
	// Photo is the CV photo (JPEG or PNG, PhotoFormat "jpeg" or "png"). Classic and modern show it in a circle;
	// compact never does.
	Photo       []byte
	PhotoFormat string
}

// DefaultExporters is the registry keyed by format.
func DefaultExporters() map[string]Exporter {
	return map[string]Exporter{
		"pdf":  NewPDFExporter(),
		"docx": NewDOCXExporter(),
	}
}
