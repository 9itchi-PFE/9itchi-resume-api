package repositories

import (
	"context"
	"time"

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
	// CountExportsByUser counts the user's downloads since a time (nil: ever); deleting a CV does not reset it.
	CountExportsByUser(ctx context.Context, userID string, since *time.Time) (int, error)
	// LatestPhoto returns the CV photo with its blob (ErrNotFound when none).
	LatestPhoto(ctx context.Context, resumeID string) (*db.ResumeFile, error)
	// HasPhoto reports whether the CV has a photo, without reading the blob.
	HasPhoto(ctx context.Context, resumeID string) (bool, error)
	// DeletePhotos removes the CV photo(s).
	DeletePhotos(ctx context.Context, resumeID string) error
	// RecordDownload logs one download for the quota.
	RecordDownload(ctx context.Context, userID, resumeID, format string) error
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

func (r *fileRepository) CountExportsByUser(ctx context.Context, userID string, since *time.Time) (int, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	var n int
	err := r.store.Conn(ctx).QueryRow(ctx,
		`SELECT count(*) FROM resume_downloads WHERE user_id = $1 AND ($2::timestamptz IS NULL OR created_at >= $2)`, userID, since).Scan(&n)
	return n, MapError(err)
}

func (r *fileRepository) RecordDownload(ctx context.Context, userID, resumeID, format string) error {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	_, err := r.store.Conn(ctx).Exec(ctx,
		`INSERT INTO resume_downloads (user_id, resume_id, format) VALUES ($1, $2, $3)`, userID, resumeID, format)
	return MapError(err)
}

func (r *fileRepository) LatestPhoto(ctx context.Context, resumeID string) (*db.ResumeFile, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`SELECT `+fileMetaColumns+`, data, updated_at FROM resume_files
		 WHERE resume_id = $1 AND kind = 'photo' ORDER BY created_at DESC LIMIT 1`, resumeID)
	if err != nil {
		return nil, MapError(err)
	}
	f, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.ResumeFile])
	return f, MapError(err)
}

func (r *fileRepository) HasPhoto(ctx context.Context, resumeID string) (bool, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	var ok bool
	err := r.store.Conn(ctx).QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM resume_files WHERE resume_id = $1 AND kind = 'photo')`, resumeID).Scan(&ok)
	return ok, MapError(err)
}

func (r *fileRepository) DeletePhotos(ctx context.Context, resumeID string) error {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	_, err := r.store.Conn(ctx).Exec(ctx, `DELETE FROM resume_files WHERE resume_id = $1 AND kind = 'photo'`, resumeID)
	return MapError(err)
}
