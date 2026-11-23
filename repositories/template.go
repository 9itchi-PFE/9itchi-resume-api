package repositories

// TemplateRepository is the only layer that touches PostgreSQL for template data.
// Use store.Conn(ctx), a context from QueryTimeout, and return MapError(err).
type TemplateRepository interface {
}

type templateRepository struct {
	store *Store
}

func NewTemplateRepository(store *Store) TemplateRepository {
	return &templateRepository{store: store}
}
