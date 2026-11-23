package repositories

// FileRepository is the only layer that touches PostgreSQL for file data.
// Use store.Conn(ctx), a context from QueryTimeout, and return MapError(err).
type FileRepository interface {
}

type fileRepository struct {
	store *Store
}

func NewFileRepository(store *Store) FileRepository {
	return &fileRepository{store: store}
}
