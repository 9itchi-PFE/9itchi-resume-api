package repositories

import (
	"context"

	"9itchi-resume-api/models/db"

	"github.com/jackc/pgx/v5"
)

const (
	resumeColumns        = `id, user_id, title, content, template_id, version, parse_status, language, parse_report, created_at, updated_at`
	resumeSummaryColumns = `id, title, template_id, version, created_at, updated_at`
)

// ResumeRepository is the only layer that touches PostgreSQL for resume data.
// Every method that takes userID scopes the query to that owner; other users' rows are ErrNotFound.
type ResumeRepository interface {
	Create(ctx context.Context, userID, title string, templateID *string, content []byte) (*db.Resume, error)
	Get(ctx context.Context, userID, id string) (*db.Resume, error)
	// Exists returns ErrNotFound when the resume does not exist or belongs to someone else.
	Exists(ctx context.Context, userID, id string) error
	// List never selects the content column.
	List(ctx context.Context, userID string, limit, offset int) ([]db.ResumeSummary, int, error)
	// Update changes only non-nil arguments.
	Update(ctx context.Context, userID, id string, title, templateID *string, content []byte) (*db.Resume, error)
	Delete(ctx context.Context, userID, id string) error
	// BumpVersion increments resumes.version and returns the new number and the current content.
	BumpVersion(ctx context.Context, userID, id string) (int, []byte, error)
	InsertVersion(ctx context.Context, resumeID string, versionNo int, content []byte) (*db.ResumeVersionMeta, error)
	// ListVersions never selects the content column.
	ListVersions(ctx context.Context, resumeID string) ([]db.ResumeVersionMeta, error)
	// CountByUser returns how many resumes the user has (free plan quota).
	CountByUser(ctx context.Context, userID string) (int, error)
	// SetParseStatus changes parse_status (background AI reading, no user scoping).
	SetParseStatus(ctx context.Context, id, status string) error
	// ApplyParsed stores the AI result only while parse_status is still pending (the user has not edited since).
	// It returns false when the result was discarded.
	ApplyParsed(ctx context.Context, id string, content []byte, language string, report []byte) (bool, error)
	// GetVersion returns one saved version with its content.
	GetVersion(ctx context.Context, resumeID, versionID string) (*db.ResumeVersion, error)
}

type resumeRepository struct {
	store *Store
}

func NewResumeRepository(store *Store) ResumeRepository {
	return &resumeRepository{store: store}
}

func (r *resumeRepository) Create(ctx context.Context, userID, title string, templateID *string, content []byte) (*db.Resume, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`INSERT INTO resumes (user_id, title, template_id, content) VALUES ($1, $2, $3, $4)
		 RETURNING `+resumeColumns, userID, title, templateID, content)
	if err != nil {
		return nil, MapError(err)
	}
	res, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.Resume])
	return res, MapError(err)
}

func (r *resumeRepository) Get(ctx context.Context, userID, id string) (*db.Resume, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`SELECT `+resumeColumns+` FROM resumes WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return nil, MapError(err)
	}
	res, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.Resume])
	return res, MapError(err)
}

func (r *resumeRepository) Exists(ctx context.Context, userID, id string) error {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	var one int
	err := r.store.Conn(ctx).QueryRow(ctx,
		`SELECT 1 FROM resumes WHERE id = $1 AND user_id = $2`, id, userID).Scan(&one)
	return MapError(err)
}

func (r *resumeRepository) List(ctx context.Context, userID string, limit, offset int) ([]db.ResumeSummary, int, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	var total int
	if err := r.store.Conn(ctx).QueryRow(ctx,
		`SELECT count(*) FROM resumes WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, MapError(err)
	}
	rows, err := r.store.Conn(ctx).Query(ctx,
		`SELECT `+resumeSummaryColumns+` FROM resumes WHERE user_id = $1
		 ORDER BY updated_at DESC, id LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, 0, MapError(err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[db.ResumeSummary])
	return items, total, MapError(err)
}

func (r *resumeRepository) Update(ctx context.Context, userID, id string, title, templateID *string, content []byte) (*db.Resume, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`UPDATE resumes SET
		   title = COALESCE($3, title),
		   template_id = COALESCE($4::uuid, template_id),
		   content = COALESCE($5::jsonb, content),
		   parse_status = CASE WHEN $5::jsonb IS NOT NULL AND parse_status = 'pending' THEN 'none' ELSE parse_status END,
		   updated_at = now()
		 WHERE id = $1 AND user_id = $2 RETURNING `+resumeColumns, id, userID, title, templateID, content)
	if err != nil {
		return nil, MapError(err)
	}
	res, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.Resume])
	return res, MapError(err)
}

func (r *resumeRepository) Delete(ctx context.Context, userID, id string) error {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	tag, err := r.store.Conn(ctx).Exec(ctx,
		`DELETE FROM resumes WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return MapError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *resumeRepository) BumpVersion(ctx context.Context, userID, id string) (int, []byte, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	var version int
	var content []byte
	err := r.store.Conn(ctx).QueryRow(ctx,
		`UPDATE resumes SET version = version + 1, updated_at = now()
		 WHERE id = $1 AND user_id = $2 RETURNING version, content`, id, userID).Scan(&version, &content)
	return version, content, MapError(err)
}

func (r *resumeRepository) InsertVersion(ctx context.Context, resumeID string, versionNo int, content []byte) (*db.ResumeVersionMeta, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`INSERT INTO resume_versions (resume_id, version_no, content) VALUES ($1, $2, $3)
		 RETURNING id, resume_id, version_no, created_at`, resumeID, versionNo, content)
	if err != nil {
		return nil, MapError(err)
	}
	v, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.ResumeVersionMeta])
	return v, MapError(err)
}

func (r *resumeRepository) ListVersions(ctx context.Context, resumeID string) ([]db.ResumeVersionMeta, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`SELECT id, resume_id, version_no, created_at FROM resume_versions
		 WHERE resume_id = $1 ORDER BY version_no DESC`, resumeID)
	if err != nil {
		return nil, MapError(err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[db.ResumeVersionMeta])
	return items, MapError(err)
}

func (r *resumeRepository) CountByUser(ctx context.Context, userID string) (int, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	var n int
	err := r.store.Conn(ctx).QueryRow(ctx, `SELECT count(*) FROM resumes WHERE user_id = $1`, userID).Scan(&n)
	return n, MapError(err)
}

func (r *resumeRepository) SetParseStatus(ctx context.Context, id, status string) error {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	tag, err := r.store.Conn(ctx).Exec(ctx, `UPDATE resumes SET parse_status = $2 WHERE id = $1`, id, status)
	if err != nil {
		return MapError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *resumeRepository) ApplyParsed(ctx context.Context, id string, content []byte, language string, report []byte) (bool, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	tag, err := r.store.Conn(ctx).Exec(ctx,
		`UPDATE resumes SET content = $2, language = NULLIF($3, ''), parse_report = $4, parse_status = 'done', updated_at = now()
		 WHERE id = $1 AND parse_status = 'pending'`, id, content, language, report)
	if err != nil {
		return false, MapError(err)
	}
	return tag.RowsAffected() == 1, nil
}

func (r *resumeRepository) GetVersion(ctx context.Context, resumeID, versionID string) (*db.ResumeVersion, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`SELECT id, resume_id, version_no, content, created_at FROM resume_versions WHERE id = $1 AND resume_id = $2`,
		versionID, resumeID)
	if err != nil {
		return nil, MapError(err)
	}
	v, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.ResumeVersion])
	return v, MapError(err)
}
