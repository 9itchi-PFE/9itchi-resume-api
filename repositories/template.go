package repositories

import (
	"context"

	"9itchi-resume-api/models/db"

	"github.com/jackc/pgx/v5"
)

const templateColumns = `id, name, layout, preview_url, active, created_at, updated_at`

// TemplatePatch holds the optional fields of an update; nil means unchanged.
type TemplatePatch struct {
	Name       *string
	Layout     *string
	PreviewURL *string
	Active     *bool
}

// TemplateRepository is the only layer that touches PostgreSQL for template data.
type TemplateRepository interface {
	ListActive(ctx context.Context) ([]db.Template, error)
	Get(ctx context.Context, id string) (*db.Template, error)
	Create(ctx context.Context, name, layout, previewURL string, active bool) (*db.Template, error)
	Update(ctx context.Context, id string, patch TemplatePatch) (*db.Template, error)
}

type templateRepository struct {
	store *Store
}

func NewTemplateRepository(store *Store) TemplateRepository {
	return &templateRepository{store: store}
}

func (r *templateRepository) ListActive(ctx context.Context) ([]db.Template, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`SELECT `+templateColumns+` FROM templates WHERE active ORDER BY name`)
	if err != nil {
		return nil, MapError(err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[db.Template])
	return items, MapError(err)
}

func (r *templateRepository) Get(ctx context.Context, id string) (*db.Template, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`SELECT `+templateColumns+` FROM templates WHERE id = $1`, id)
	if err != nil {
		return nil, MapError(err)
	}
	t, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.Template])
	return t, MapError(err)
}

func (r *templateRepository) Create(ctx context.Context, name, layout, previewURL string, active bool) (*db.Template, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`INSERT INTO templates (name, layout, preview_url, active) VALUES ($1, $2, $3, $4)
		 RETURNING `+templateColumns, name, layout, previewURL, active)
	if err != nil {
		return nil, MapError(err)
	}
	t, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.Template])
	return t, MapError(err)
}

func (r *templateRepository) Update(ctx context.Context, id string, p TemplatePatch) (*db.Template, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`UPDATE templates SET
		   name = COALESCE($2, name),
		   layout = COALESCE($3, layout),
		   preview_url = COALESCE($4, preview_url),
		   active = COALESCE($5, active),
		   updated_at = now()
		 WHERE id = $1 RETURNING `+templateColumns, id, p.Name, p.Layout, p.PreviewURL, p.Active)
	if err != nil {
		return nil, MapError(err)
	}
	t, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.Template])
	return t, MapError(err)
}
