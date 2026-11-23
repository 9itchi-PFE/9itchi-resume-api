package services

import (
	"context"

	"9itchi-resume-api/models"
	"9itchi-resume-api/models/api"
	"9itchi-resume-api/repositories"
)

// ResumeService holds all business logic for resume. Handlers stay thin.
type ResumeService interface {
	// POST /resumes/upload: upload PDF/DOCX, async AI parse
	UploadResume(ctx context.Context, userID string) (any, *models.ApiError)
	// POST /resumes: create resume from structured content
	CreateResume(ctx context.Context, userID string, req api.CreateResumeRequest) (any, *models.ApiError)
	// GET /resumes: list own resumes
	ListResumes(ctx context.Context, userID string) (any, *models.ApiError)
	// GET /resumes/:id: resume detail
	GetResume(ctx context.Context, userID string, id string) (any, *models.ApiError)
	// PUT /resumes/:id: edit sections / switch template
	UpdateResume(ctx context.Context, userID string, id string, req api.UpdateResumeRequest) (any, *models.ApiError)
	// DELETE /resumes/:id: delete resume
	DeleteResume(ctx context.Context, userID string, id string) (any, *models.ApiError)
	// GET /resumes/:id/versions: list saved versions
	ListVersions(ctx context.Context, userID string, id string) (any, *models.ApiError)
	// POST /resumes/:id/versions: snapshot a version
	CreateVersion(ctx context.Context, userID string, id string) (any, *models.ApiError)
	// POST /resumes/:id/export: export PDF or DOCX
	ExportResume(ctx context.Context, userID string, id string, req api.ExportResumeRequest) (any, *models.ApiError)
	// POST /resumes/:id/ats-check: ATS score and suggestions
	ATSCheck(ctx context.Context, userID string, id string) (any, *models.ApiError)
	// GET /internal/resumes/:id/pdf: latest exported PDF bytes (email attachments)
	InternalResumePDF(ctx context.Context, id string) (any, *models.ApiError)
}

type resumeService struct {
	repo repositories.ResumeRepository
}

func NewResumeService(repo repositories.ResumeRepository) ResumeService {
	return &resumeService{repo: repo}
}

func (s *resumeService) UploadResume(_ context.Context, _ string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *resumeService) CreateResume(_ context.Context, _ string, _ api.CreateResumeRequest) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *resumeService) ListResumes(_ context.Context, _ string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *resumeService) GetResume(_ context.Context, _ string, _ string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *resumeService) UpdateResume(_ context.Context, _ string, _ string, _ api.UpdateResumeRequest) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *resumeService) DeleteResume(_ context.Context, _ string, _ string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *resumeService) ListVersions(_ context.Context, _ string, _ string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *resumeService) CreateVersion(_ context.Context, _ string, _ string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *resumeService) ExportResume(_ context.Context, _ string, _ string, _ api.ExportResumeRequest) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *resumeService) ATSCheck(_ context.Context, _ string, _ string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *resumeService) InternalResumePDF(_ context.Context, _ string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}
