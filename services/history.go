package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"time"

	"9itchi-resume-api/core"
	"9itchi-resume-api/models"
	"9itchi-resume-api/models/api"
	"9itchi-resume-api/models/db"
	"9itchi-resume-api/models/mapping"
)

// MaxPhotoBytes caps the CV photo.
const MaxPhotoBytes = 2 << 20

// GetVersion returns one saved version with its content (open it from the history tab).
func (s *resumeService) GetVersion(ctx context.Context, userID, id, versionID string) (*api.VersionContentResponse, *models.ApiError) {
	if !isUUID(id) || !isUUID(versionID) {
		return nil, newError(http.StatusBadRequest, "invalid id")
	}
	if err := s.repo.Exists(ctx, userID, id); err != nil {
		return nil, repoError(err, "resume not found")
	}
	v, err := s.repo.GetVersion(ctx, id, versionID)
	if err != nil {
		return nil, repoError(err, "version not found")
	}
	return &api.VersionContentResponse{
		VersionResponse: api.VersionResponse{ID: v.ID, VersionNo: v.VersionNo, Label: v.Label, CreatedAt: v.CreatedAt},
		Content:         mapping.ParseContent(v.Content),
	}, nil
}

// RestoreVersion puts a saved version back as the current content. The content it replaces is saved first,
// so a restore can always be undone from the history.
// LabelBeforeRestore marks the version saved just before a restore, so the restore can be undone.
const LabelBeforeRestore = "before_restore"

func (s *resumeService) RestoreVersion(ctx context.Context, userID, id, versionID string) (*api.ResumeResponse, *models.ApiError) {
	if !isUUID(id) || !isUUID(versionID) {
		return nil, newError(http.StatusBadRequest, "invalid id")
	}
	if apiErr := s.checkEditable(ctx, userID, id); apiErr != nil {
		return nil, apiErr
	}
	var res *db.Resume
	err := s.tx.WithTx(ctx, func(ctx context.Context) error {
		v, err := s.repo.GetVersion(ctx, id, versionID)
		if err != nil {
			return err
		}
		versionNo, current, err := s.repo.BumpVersion(ctx, userID, id)
		if err != nil {
			return err
		}
		if _, err := s.repo.InsertVersion(ctx, id, versionNo, LabelBeforeRestore, current); err != nil {
			return err
		}
		res, err = s.repo.Update(ctx, userID, id, nil, nil, v.Content)
		return err
	})
	if err != nil {
		return nil, repoError(err, "version not found")
	}
	return s.response(ctx, res), nil
}

// DuplicateResume copies a CV (content and template) under a new title: one version per kind of job.
// A copy is not an import: it never counts against the plan's CV limit.
func (s *resumeService) DuplicateResume(ctx context.Context, userID, id string, req api.DuplicateResumeRequest) (*api.ResumeResponse, *models.ApiError) {
	if !isUUID(id) {
		return nil, newError(http.StatusBadRequest, "invalid resume id")
	}
	if apiErr := validateTitle(req.Title); apiErr != nil {
		return nil, apiErr
	}
	if !s.canEdit(ctx, userID) {
		return nil, newError(http.StatusPaymentRequired, errEditNeedsPlan)
	}
	src, err := s.repo.Get(ctx, userID, id)
	if err != nil {
		return nil, repoError(err, "resume not found")
	}
	res, err := s.repo.Duplicate(ctx, userID, req.Title, src)
	if err != nil {
		return nil, repoError(err, "resume not found")
	}
	return s.response(ctx, res), nil
}

// UploadPhoto stores the CV photo (JPEG or PNG, 2 MB max), replacing the previous one.
func (s *resumeService) UploadPhoto(ctx context.Context, userID, id string, r io.Reader) *models.ApiError {
	if !isUUID(id) {
		return newError(http.StatusBadRequest, "invalid resume id")
	}
	data, err := io.ReadAll(io.LimitReader(r, MaxPhotoBytes+1))
	if err != nil {
		return newError(http.StatusBadRequest, "could not read file")
	}
	if len(data) > MaxPhotoBytes {
		return newError(http.StatusRequestEntityTooLarge, "photo too large (max 2 MB)")
	}
	var format, contentType string
	switch {
	case bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}):
		format, contentType = "jpeg", "image/jpeg"
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		format, contentType = "png", "image/png"
	default:
		return newError(http.StatusBadRequest, "only JPEG or PNG photos are accepted")
	}
	if err := s.repo.Exists(ctx, userID, id); err != nil {
		return repoError(err, "resume not found")
	}
	if apiErr := s.checkEditable(ctx, userID, id); apiErr != nil {
		return apiErr
	}
	sum := sha256.Sum256(data)
	err = s.tx.WithTx(ctx, func(ctx context.Context) error {
		if err := s.files.DeletePhotos(ctx, id); err != nil {
			return err
		}
		_, err := s.files.Create(ctx, &db.ResumeFile{
			ResumeID: id, Kind: "photo", Format: format, Filename: "photo." + format, ContentType: contentType,
			SizeBytes: int64(len(data)), SHA256: hex.EncodeToString(sum[:]), Data: data,
		})
		return err
	})
	if err != nil {
		return repoError(err, "resume not found")
	}
	return nil
}

// GetPhoto returns the CV photo.
func (s *resumeService) GetPhoto(ctx context.Context, userID, id string) (*api.FileContent, *models.ApiError) {
	if !isUUID(id) {
		return nil, newError(http.StatusBadRequest, "invalid resume id")
	}
	if err := s.repo.Exists(ctx, userID, id); err != nil {
		return nil, repoError(err, "resume not found")
	}
	f, err := s.files.LatestPhoto(ctx, id)
	if err != nil {
		return nil, repoError(err, "no photo")
	}
	return &api.FileContent{Filename: f.Filename, ContentType: f.ContentType, Data: f.Data}, nil
}

// DeletePhoto removes the CV photo.
func (s *resumeService) DeletePhoto(ctx context.Context, userID, id string) *models.ApiError {
	if !isUUID(id) {
		return newError(http.StatusBadRequest, "invalid resume id")
	}
	if err := s.repo.Exists(ctx, userID, id); err != nil {
		return repoError(err, "resume not found")
	}
	if apiErr := s.checkEditable(ctx, userID, id); apiErr != nil {
		return apiErr
	}
	if err := s.files.DeletePhotos(ctx, id); err != nil {
		return repoError(err, "resume not found")
	}
	return nil
}

// response maps a resume and adds whether it has a photo.
func (s *resumeService) response(ctx context.Context, res *db.Resume) *api.ResumeResponse {
	resp := mapping.ResumeToResponse(res)
	if ok, err := s.files.HasPhoto(ctx, res.ID); err == nil {
		resp.HasPhoto = ok
	}
	return &resp
}

func (s *resumeService) Downloads(ctx context.Context, userID string) (*api.DownloadsResponse, *models.ApiError) {
	q, apiErr := s.plan(ctx, userID)
	if apiErr != nil {
		return nil, apiErr
	}
	var since *time.Time
	out := &api.DownloadsResponse{}
	if q != nil {
		since = q.PeriodStart
		if limit, ok := q.Limits[core.QuotaCVDownloads]; ok {
			out.Limit = &limit
		}
	}
	n, err := s.files.CountExportsByUser(ctx, userID, since)
	if err != nil {
		return nil, repoError(err, "not found")
	}
	out.Used = n
	return out, nil
}
