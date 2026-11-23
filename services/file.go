package services

import (
	"context"

	"9itchi-resume-api/models"
	"9itchi-resume-api/repositories"
)

// FileService holds all business logic for file. Handlers stay thin.
type FileService interface {
	// GET /resumes/:id/files: file metadata, never the blob
	ListFiles(ctx context.Context, userID string, id string) (any, *models.ApiError)
	// GET /resumes/:id/files/:fileId: stream the stored PDF/DOCX blob
	DownloadFile(ctx context.Context, userID string, id string, fileId string) (any, *models.ApiError)
	// DELETE /resumes/:id/files/:fileId: delete a stored file
	DeleteFile(ctx context.Context, userID string, id string, fileId string) (any, *models.ApiError)
}

type fileService struct {
	repo repositories.FileRepository
}

func NewFileService(repo repositories.FileRepository) FileService {
	return &fileService{repo: repo}
}

func (s *fileService) ListFiles(_ context.Context, _ string, _ string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *fileService) DownloadFile(_ context.Context, _ string, _ string, _ string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *fileService) DeleteFile(_ context.Context, _ string, _ string, _ string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}
