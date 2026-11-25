package repositories

import (
	"context"

	"9itchi-resume-api/utils"
)

// Repositories holds every repository plus the shared Store (pool + transactions).
type Repositories struct {
	Store    *Store
	Resume   ResumeRepository
	Template TemplateRepository
	File     FileRepository
	Tip      TipRepository
	Erase    EraseRepository
}

// InitRepositories connects to PostgreSQL, applies migrations and builds the repositories.
func InitRepositories(ctx context.Context, config utils.AppConfig) (*Repositories, error) {
	if err := RunMigrations(ctx, config.DatabaseURL); err != nil {
		return nil, err
	}
	store, err := NewStore(ctx, config.DatabaseURL)
	if err != nil {
		return nil, err
	}
	return &Repositories{
		Store:    store,
		Resume:   NewResumeRepository(store),
		Template: NewTemplateRepository(store),
		File:     NewFileRepository(store),
		Tip:      NewTipRepository(store),
		Erase:    NewEraseRepository(store),
	}, nil
}

func (r *Repositories) Close() { r.Store.Close() }
