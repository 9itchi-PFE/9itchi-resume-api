// Package mocks holds hand-written repository mocks. Set the *Fn fields a test needs;
// an unset method returns zero values.
package mocks

import (
	"context"
	"time"

	"9itchi-resume-api/models/db"
	"9itchi-resume-api/repositories"
)

// Tx runs the function directly (no real transaction).
type Tx struct{ Err error }

func (t Tx) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if t.Err != nil {
		return t.Err
	}
	return fn(ctx)
}

type ResumeRepo struct {
	CreateFn         func(ctx context.Context, userID, title string, templateID *string, content []byte) (*db.Resume, error)
	GetFn            func(ctx context.Context, userID, id string) (*db.Resume, error)
	GetByIDFn        func(ctx context.Context, id string) (*db.Resume, error)
	ExistsFn         func(ctx context.Context, userID, id string) error
	ListFn           func(ctx context.Context, userID string, limit, offset int) ([]db.ResumeSummary, int, error)
	UpdateFn         func(ctx context.Context, userID, id string, title, templateID *string, content []byte) (*db.Resume, error)
	DeleteFn         func(ctx context.Context, userID, id string) error
	BumpVersionFn    func(ctx context.Context, userID, id string) (int, []byte, error)
	InsertVersionFn  func(ctx context.Context, resumeID string, versionNo int, label string, content []byte) (*db.ResumeVersionMeta, error)
	ListVersionsFn   func(ctx context.Context, resumeID string) ([]db.ResumeVersionMeta, error)
	CountByUserFn    func(ctx context.Context, userID string) (int, error)
	SetParseStatusFn func(ctx context.Context, id, status string) error
	ApplyParsedFn    func(ctx context.Context, id string, content []byte, language string, report []byte) (bool, error)
	GetVersionFn     func(ctx context.Context, resumeID, versionID string) (*db.ResumeVersion, error)
}

func (m *ResumeRepo) GetVersion(ctx context.Context, resumeID, versionID string) (*db.ResumeVersion, error) {
	if m.GetVersionFn == nil {
		return nil, nil
	}
	return m.GetVersionFn(ctx, resumeID, versionID)
}

func (m *ResumeRepo) CountByUser(ctx context.Context, userID string) (int, error) {
	if m.CountByUserFn == nil {
		return 0, nil
	}
	return m.CountByUserFn(ctx, userID)
}

func (m *ResumeRepo) SetParseStatus(ctx context.Context, id, status string) error {
	if m.SetParseStatusFn == nil {
		return nil
	}
	return m.SetParseStatusFn(ctx, id, status)
}

func (m *ResumeRepo) ApplyParsed(ctx context.Context, id string, content []byte, language string, report []byte) (bool, error) {
	if m.ApplyParsedFn == nil {
		return true, nil
	}
	return m.ApplyParsedFn(ctx, id, content, language, report)
}

var _ repositories.ResumeRepository = (*ResumeRepo)(nil)

func (m *ResumeRepo) Create(ctx context.Context, userID, title string, templateID *string, content []byte) (*db.Resume, error) {
	if m.CreateFn == nil {
		return nil, nil
	}
	return m.CreateFn(ctx, userID, title, templateID, content)
}

func (m *ResumeRepo) Get(ctx context.Context, userID, id string) (*db.Resume, error) {
	if m.GetFn == nil {
		return nil, nil
	}
	return m.GetFn(ctx, userID, id)
}

func (m *ResumeRepo) GetByID(ctx context.Context, id string) (*db.Resume, error) {
	if m.GetByIDFn == nil {
		return nil, repositories.ErrNotFound
	}
	return m.GetByIDFn(ctx, id)
}

func (m *ResumeRepo) Exists(ctx context.Context, userID, id string) error {
	if m.ExistsFn == nil {
		return nil
	}
	return m.ExistsFn(ctx, userID, id)
}

func (m *ResumeRepo) List(ctx context.Context, userID string, limit, offset int) ([]db.ResumeSummary, int, error) {
	if m.ListFn == nil {
		return nil, 0, nil
	}
	return m.ListFn(ctx, userID, limit, offset)
}

func (m *ResumeRepo) Update(ctx context.Context, userID, id string, title, templateID *string, content []byte) (*db.Resume, error) {
	if m.UpdateFn == nil {
		return nil, nil
	}
	return m.UpdateFn(ctx, userID, id, title, templateID, content)
}

func (m *ResumeRepo) Delete(ctx context.Context, userID, id string) error {
	if m.DeleteFn == nil {
		return nil
	}
	return m.DeleteFn(ctx, userID, id)
}

func (m *ResumeRepo) BumpVersion(ctx context.Context, userID, id string) (int, []byte, error) {
	if m.BumpVersionFn == nil {
		return 0, nil, nil
	}
	return m.BumpVersionFn(ctx, userID, id)
}

func (m *ResumeRepo) InsertVersion(ctx context.Context, resumeID string, versionNo int, label string, content []byte) (*db.ResumeVersionMeta, error) {
	if m.InsertVersionFn == nil {
		return nil, nil
	}
	return m.InsertVersionFn(ctx, resumeID, versionNo, label, content)
}

func (m *ResumeRepo) ListVersions(ctx context.Context, resumeID string) ([]db.ResumeVersionMeta, error) {
	if m.ListVersionsFn == nil {
		return nil, nil
	}
	return m.ListVersionsFn(ctx, resumeID)
}

type TemplateRepo struct {
	ListActiveFn func(ctx context.Context) ([]db.Template, error)
	ListAllFn    func(ctx context.Context) ([]db.TemplateUsage, error)
	OtherActive  *int
	GetFn        func(ctx context.Context, id string) (*db.Template, error)
	CreateFn     func(ctx context.Context, name, layout, previewURL string, active bool) (*db.Template, error)
	UpdateFn     func(ctx context.Context, id string, patch repositories.TemplatePatch) (*db.Template, error)
}

var _ repositories.TemplateRepository = (*TemplateRepo)(nil)

func (m *TemplateRepo) ListAll(ctx context.Context) ([]db.TemplateUsage, error) {
	if m.ListAllFn == nil {
		return nil, nil
	}
	return m.ListAllFn(ctx)
}

// CountOtherActive answers 1 unless OtherActive says otherwise: another template stays published.
func (m *TemplateRepo) CountOtherActive(context.Context, string) (int, error) {
	if m.OtherActive == nil {
		return 1, nil
	}
	return *m.OtherActive, nil
}

func (m *TemplateRepo) ListActive(ctx context.Context) ([]db.Template, error) {
	if m.ListActiveFn == nil {
		return nil, nil
	}
	return m.ListActiveFn(ctx)
}

func (m *TemplateRepo) Get(ctx context.Context, id string) (*db.Template, error) {
	if m.GetFn == nil {
		return nil, repositories.ErrNotFound
	}
	return m.GetFn(ctx, id)
}

func (m *TemplateRepo) Create(ctx context.Context, name, layout, previewURL string, active bool) (*db.Template, error) {
	if m.CreateFn == nil {
		return nil, nil
	}
	return m.CreateFn(ctx, name, layout, previewURL, active)
}

func (m *TemplateRepo) Update(ctx context.Context, id string, patch repositories.TemplatePatch) (*db.Template, error) {
	if m.UpdateFn == nil {
		return nil, nil
	}
	return m.UpdateFn(ctx, id, patch)
}

type FileRepo struct {
	CreateFn             func(ctx context.Context, f *db.ResumeFile) (*db.ResumeFileMeta, error)
	ListByResumeFn       func(ctx context.Context, resumeID string) ([]db.ResumeFileMeta, error)
	GetFn                func(ctx context.Context, resumeID, fileID string) (*db.ResumeFile, error)
	DeleteFn             func(ctx context.Context, resumeID, fileID string) error
	LatestExportFn       func(ctx context.Context, resumeID, format string) (*db.ResumeFile, error)
	OriginalFn           func(ctx context.Context, resumeID string) (*db.ResumeFile, error)
	CountExportsByUserFn func(ctx context.Context, userID string, since *time.Time) (int, error)
	LastExportByUserFn   func(ctx context.Context, userID string) (int, *time.Time, error)
	RecordDownloadFn     func(ctx context.Context, userID, resumeID, format string) error
	LatestPhotoFn        func(ctx context.Context, resumeID string) (*db.ResumeFile, error)
	HasPhotoFn           func(ctx context.Context, resumeID string) (bool, error)
	DeletePhotosFn       func(ctx context.Context, resumeID string) error
}

func (m *FileRepo) LatestPhoto(ctx context.Context, resumeID string) (*db.ResumeFile, error) {
	if m.LatestPhotoFn == nil {
		return nil, repositories.ErrNotFound
	}
	return m.LatestPhotoFn(ctx, resumeID)
}

func (m *FileRepo) HasPhoto(ctx context.Context, resumeID string) (bool, error) {
	if m.HasPhotoFn == nil {
		return false, nil
	}
	return m.HasPhotoFn(ctx, resumeID)
}

func (m *FileRepo) DeletePhotos(ctx context.Context, resumeID string) error {
	if m.DeletePhotosFn == nil {
		return nil
	}
	return m.DeletePhotosFn(ctx, resumeID)
}

func (m *FileRepo) RecordDownload(ctx context.Context, userID, resumeID, format string) error {
	if m.RecordDownloadFn == nil {
		return nil
	}
	return m.RecordDownloadFn(ctx, userID, resumeID, format)
}

func (m *FileRepo) CountExportsByUser(ctx context.Context, userID string, since *time.Time) (int, error) {
	if m.CountExportsByUserFn == nil {
		return 0, nil
	}
	return m.CountExportsByUserFn(ctx, userID, since)
}

func (m *FileRepo) LastExportByUser(ctx context.Context, userID string) (int, *time.Time, error) {
	if m.LastExportByUserFn == nil {
		return 0, nil, nil
	}
	return m.LastExportByUserFn(ctx, userID)
}

var _ repositories.FileRepository = (*FileRepo)(nil)

func (m *FileRepo) Create(ctx context.Context, f *db.ResumeFile) (*db.ResumeFileMeta, error) {
	if m.CreateFn == nil {
		return nil, nil
	}
	return m.CreateFn(ctx, f)
}

func (m *FileRepo) ListByResume(ctx context.Context, resumeID string) ([]db.ResumeFileMeta, error) {
	if m.ListByResumeFn == nil {
		return nil, nil
	}
	return m.ListByResumeFn(ctx, resumeID)
}

func (m *FileRepo) Get(ctx context.Context, resumeID, fileID string) (*db.ResumeFile, error) {
	if m.GetFn == nil {
		return nil, repositories.ErrNotFound
	}
	return m.GetFn(ctx, resumeID, fileID)
}

func (m *FileRepo) Delete(ctx context.Context, resumeID, fileID string) error {
	if m.DeleteFn == nil {
		return nil
	}
	return m.DeleteFn(ctx, resumeID, fileID)
}

func (m *FileRepo) LatestExport(ctx context.Context, resumeID, format string) (*db.ResumeFile, error) {
	if m.LatestExportFn == nil {
		return nil, repositories.ErrNotFound
	}
	return m.LatestExportFn(ctx, resumeID, format)
}

func (m *FileRepo) Original(ctx context.Context, resumeID string) (*db.ResumeFile, error) {
	if m.OriginalFn == nil {
		return nil, repositories.ErrNotFound
	}
	return m.OriginalFn(ctx, resumeID)
}

// TipRepo is a mock of repositories.TipRepository. It records who was released.
type TipRepo struct {
	ClaimFn    func(ctx context.Context, idleBefore, tippedBefore time.Time, limit int) ([]db.TipClaim, error)
	ReleaseErr error
	Released   []string
}

func (m *TipRepo) Claim(ctx context.Context, idleBefore, tippedBefore time.Time, limit int) ([]db.TipClaim, error) {
	if m.ClaimFn == nil {
		return nil, nil
	}
	return m.ClaimFn(ctx, idleBefore, tippedBefore, limit)
}

func (m *TipRepo) Release(_ context.Context, userID string) error {
	m.Released = append(m.Released, userID)
	return m.ReleaseErr
}
