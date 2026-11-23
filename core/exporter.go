package core

import "context"

// Exporter renders a resume to a file format. Register implementations in services.exporters.
type Exporter interface {
	Format() string
	Render(ctx context.Context, html string) ([]byte, error)
}
