package repositories

import (
	"context"

	"9itchi-resume-api/models/db"

	"github.com/jackc/pgx/v5"
)

const fileMetaColumns = `id, resume_id, kind, format, filename, content_type, size_bytes, sha256, created_at`

// FileRepository is the only layer that touches PostgreSQL for file data.
// It does not check ownership: services verify the resume owner first (ResumeRepository.Exists).
// List and metadata queries never select the data column.
type FileRepository interface {
	Create(ctx context.Context, f *db.ResumeFile) (*db.ResumeFileMeta, error)
	ListByResume(ctx context.Context, resumeID string) ([]db.ResumeFileMeta, error)
	// Get returns the file including its blob.
	Get(ctx context.Context, resumeID, fileID string) (*db.ResumeFile, error)
	Delete(ctx context.Context, resumeID, fileID string) error
	// LatestExport returns the newest kind=export file of the given format, including its blob.
	LatestExport(ctx context.Context, resumeID, format string) (*db.ResumeFile, error)
}

type fileRepository struct {
	store *Store
}

func NewFileRepository(store *Store) FileRepository {
	return &fileRepository{store: store}
}

func (r *fileRepository) Create(ctx context.Context, f *db.ResumeFile) (*db.ResumeFileMeta, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`INSERT INTO resume_files (resume_id, kind, format, filename, content_type, size_bytes, sha256, data)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING `+fileMetaColumns,
		f.ResumeID, f.Kind, f.Format, f.Filename, f.ContentType, f.SizeBytes, f.SHA256, f.Data)
	if err != nil {
		return nil, MapError(err)
	}
	m, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.ResumeFileMeta])
	return m, MapError(err)
}

func (r *fileRepository) ListByResume(ctx context.Context, resumeID string) ([]db.ResumeFileMeta, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`SELECT `+fileMetaColumns+` FROM resume_files WHERE resume_id = $1
		 ORDER BY created_at DESC, id`, resumeID)
	if err != nil {
		return nil, MapError(err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[db.ResumeFileMeta])
	return items, MapError(err)
}

func (r *fileRepository) Get(ctx context.Context, resumeID, fileID string) (*db.ResumeFile, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`SELECT `+fileMetaColumns+`, data, updated_at FROM resume_files WHERE id = $1 AND resume_id = $2`,
		fileID, resumeID)
	if err != nil {
		return nil, MapError(err)
	}
	f, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.ResumeFile])
	return f, MapError(err)
}

func (r *fileRepository) Delete(ctx context.Context, resumeID, fileID string) error {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	tag, err := r.store.Conn(ctx).Exec(ctx,
		`DELETE FROM resume_files WHERE id = $1 AND resume_id = $2`, fileID, resumeID)
	if err != nil {
		return MapError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *fileRepository) LatestExport(ctx context.Context, resumeID, format string) (*db.ResumeFile, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`SELECT `+fileMetaColumns+`, data, updated_at FROM resume_files
		 WHERE resume_id = $1 AND kind = 'export' AND format = $2
		 ORDER BY created_at DESC LIMIT 1`, resumeID, format)
	if err != nil {
		return nil, MapError(err)
	}
	f, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.ResumeFile])
	return f, MapError(err)
}
