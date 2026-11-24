package repositories

import (
	"context"
	"time"

	"9itchi-resume-api/models/db"

	"github.com/jackc/pgx/v5"
)

// TipRepository hands out, once a week each, the students whose CV gets the weekly advice.
type TipRepository interface {
	// Claim marks as tipped now and returns, at most limit at a time, the students who were never tipped or were
	// last tipped before tippedBefore, each with the CV they changed last. A CV still being read, or changed since
	// idleBefore (the student is working on it), is left alone.
	Claim(ctx context.Context, idleBefore, tippedBefore time.Time, limit int) ([]db.TipClaim, error)
	// Release undoes a claim whose tip could not be raised, so that it is tried again.
	Release(ctx context.Context, userID string) error
}

type tipRepository struct {
	store *Store
}

func NewTipRepository(store *Store) TipRepository {
	return &tipRepository{store: store}
}

func (r *tipRepository) Claim(ctx context.Context, idleBefore, tippedBefore time.Time, limit int) ([]db.TipClaim, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	// The content column is never read here. Two instances that pick the same student meet on the primary key:
	// the second one finds sent_at fresh and updates nothing.
	rows, err := r.store.Conn(ctx).Query(ctx, `WITH latest AS (
  SELECT DISTINCT ON (user_id) user_id, id AS resume_id, updated_at
  FROM resumes
  WHERE parse_status <> 'pending'
  ORDER BY user_id, updated_at DESC
), due AS (
  SELECT l.user_id, l.resume_id
  FROM latest l LEFT JOIN cv_tips t ON t.user_id = l.user_id
  WHERE l.updated_at < $1 AND (t.sent_at IS NULL OR t.sent_at < $2)
  ORDER BY t.sent_at NULLS FIRST, l.user_id
  LIMIT $3
)
INSERT INTO cv_tips (user_id, resume_id, sent_at) SELECT user_id, resume_id, now() FROM due
ON CONFLICT (user_id) DO UPDATE SET resume_id = EXCLUDED.resume_id, sent_at = now() WHERE cv_tips.sent_at < $2
RETURNING user_id, resume_id`, idleBefore, tippedBefore, limit)
	if err != nil {
		return nil, MapError(err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByName[db.TipClaim])
	return out, MapError(err)
}

func (r *tipRepository) Release(ctx context.Context, userID string) error {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	_, err := r.store.Conn(ctx).Exec(ctx, `DELETE FROM cv_tips WHERE user_id = $1`, userID)
	return MapError(err)
}
