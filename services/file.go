package services

import (
	"context"
	"net/http"

	"9itchi-resume-api/models"
	"9itchi-resume-api/models/api"
	"9itchi-resume-api/models/mapping"
	"9itchi-resume-api/repositories"
)

// FileService holds all business logic for file. Handlers stay thin.
// Every method first checks that the resume belongs to the caller (other users get 404).
type FileService interface {
	// GET /resumes/:id/files: file metadata, never the blob
	ListFiles(ctx context.Context, userID string, id string) (*api.ListFilesResponse, *models.ApiError)
	// GET /resumes/:id/files/:fileId: the stored PDF/DOCX blob
	DownloadFile(ctx context.Context, userID string, id string, fileId string) (*api.FileContent, *models.ApiError)
	// DELETE /resumes/:id/files/:fileId: delete a stored file
	DeleteFile(ctx context.Context, userID string, id string, fileId string) *models.ApiError
}

type fileService struct {
	repo    repositories.FileRepository
	resumes repositories.ResumeRepository
}

func NewFileService(repo repositories.FileRepository, resumes repositories.ResumeRepository) FileService {
	return &fileService{repo: repo, resumes: resumes}
}

// authorize validates ids and checks resume ownership.
func (s *fileService) authorize(ctx context.Context, userID, id string, fileID *string) *models.ApiError {
	if !isUUID(id) || (fileID != nil && !isUUID(*fileID)) {
		return newError(http.StatusBadRequest, "invalid id")
	}
	if err := s.resumes.Exists(ctx, userID, id); err != nil {
		return repoError(err, "resume not found")
	}
	return nil
}

func (s *fileService) ListFiles(ctx context.Context, userID string, id string) (*api.ListFilesResponse, *models.ApiError) {
	if apiErr := s.authorize(ctx, userID, id, nil); apiErr != nil {
		return nil, apiErr
	}
	rows, err := s.repo.ListByResume(ctx, id)
	if err != nil {
		return nil, repoError(err, "file not found")
	}
	out := &api.ListFilesResponse{Items: make([]api.FileResponse, 0, len(rows))}
	for i := range rows {
		out.Items = append(out.Items, mapping.FileMetaToResponse(&rows[i]))
	}
	return out, nil
}

func (s *fileService) DownloadFile(ctx context.Context, userID string, id string, fileId string) (*api.FileContent, *models.ApiError) {
	if apiErr := s.authorize(ctx, userID, id, &fileId); apiErr != nil {
		return nil, apiErr
	}
	f, err := s.repo.Get(ctx, id, fileId)
	if err != nil {
		return nil, repoError(err, "file not found")
	}
	return &api.FileContent{Filename: sanitizeFilename(f.Filename), ContentType: f.ContentType, Data: f.Data}, nil
}

func (s *fileService) DeleteFile(ctx context.Context, userID string, id string, fileId string) *models.ApiError {
	if apiErr := s.authorize(ctx, userID, id, &fileId); apiErr != nil {
		return apiErr
	}
	if err := s.repo.Delete(ctx, id, fileId); err != nil {
		return repoError(err, "file not found")
	}
	return nil
}
