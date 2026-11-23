package repositories

// ResumeRepository is the only layer that touches PostgreSQL for resume data.
// Use store.Conn(ctx), a context from QueryTimeout, and return MapError(err).
type ResumeRepository interface {
}

type resumeRepository struct {
	store *Store
}

func NewResumeRepository(store *Store) ResumeRepository {
	return &resumeRepository{store: store}
}
