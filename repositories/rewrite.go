package repositories

import (
	"context"
	"time"

	"9itchi-resume-api/models/db"
)

// RewriteRepository keeps what the AI proposed for lines of a student's CV.
type RewriteRepository interface {
	// Find returns the proposal for exactly this input (ErrNotFound when none).
	Find(ctx context.Context, userID, inputHash string) (*db.AIRewrite, error)
	// Save stores a proposal; saving the same input again keeps the first one.
	Save(ctx context.Context, r *db.AIRewrite) (*db.AIRewrite, error)
	// CountSince counts the proposals made for the user since a time: the plan's monthly limit counts these.
	CountSince(ctx context.Context, userID string, since time.Time) (int, error)
}

type rewriteRepository struct {
	store *Store
}

func NewRewriteRepository(store *Store) RewriteRepository {
	return &rewriteRepository{store: store}
}

const rewriteColumns = `id, user_id, resume_id, input_hash, suggestions, warnings, created_at`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRewrite(r rowScanner) (*db.AIRewrite, error) {
	var w db.AIRewrite
	if err := r.Scan(&w.ID, &w.UserID, &w.ResumeID, &w.InputHash, &w.Suggestions, &w.Warnings, &w.CreatedAt); err != nil {
		return nil, MapError(err)
	}
	return &w, nil
}

func (r *rewriteRepository) Find(ctx context.Context, userID, inputHash string) (*db.AIRewrite, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	return scanRewrite(r.store.Conn(ctx).QueryRow(ctx,
		`SELECT `+rewriteColumns+` FROM ai_rewrites WHERE user_id = $1 AND input_hash = $2`, userID, inputHash))
}

func (r *rewriteRepository) Save(ctx context.Context, w *db.AIRewrite) (*db.AIRewrite, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	suggestions := w.Suggestions
	if suggestions == nil {
		suggestions = []string{}
	}
	warnings := w.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	return scanRewrite(r.store.Conn(ctx).QueryRow(ctx, `
		INSERT INTO ai_rewrites (user_id, resume_id, input_hash, suggestions, warnings) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, input_hash) DO UPDATE SET input_hash = ai_rewrites.input_hash
		RETURNING `+rewriteColumns, w.UserID, w.ResumeID, w.InputHash, suggestions, warnings))
}

func (r *rewriteRepository) CountSince(ctx context.Context, userID string, since time.Time) (int, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	var n int
	err := r.store.Conn(ctx).QueryRow(ctx, `SELECT count(*) FROM ai_rewrites WHERE user_id = $1 AND created_at >= $2`, userID, since).Scan(&n)
	return n, MapError(err)
}
