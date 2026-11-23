package services

import (
	"context"

	"9itchi-resume-api/models"
	"9itchi-resume-api/models/api"
	"9itchi-resume-api/repositories"
)

// TemplateService holds all business logic for template. Handlers stay thin.
type TemplateService interface {
	// GET /templates: template gallery
	ListTemplates(ctx context.Context, userID string) (any, *models.ApiError)
	// GET /templates/:id: template detail
	GetTemplate(ctx context.Context, userID string, id string) (any, *models.ApiError)
	// POST /admin/templates: add template
	CreateTemplate(ctx context.Context, userID string, req api.CreateTemplateRequest) (any, *models.ApiError)
	// PUT /admin/templates/:id: edit or deactivate template
	UpdateTemplate(ctx context.Context, userID string, id string, req api.UpdateTemplateRequest) (any, *models.ApiError)
}

type templateService struct {
	repo repositories.TemplateRepository
}

func NewTemplateService(repo repositories.TemplateRepository) TemplateService {
	return &templateService{repo: repo}
}

func (s *templateService) ListTemplates(_ context.Context, _ string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *templateService) GetTemplate(_ context.Context, _ string, _ string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *templateService) CreateTemplate(_ context.Context, _ string, _ api.CreateTemplateRequest) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *templateService) UpdateTemplate(_ context.Context, _ string, _ string, _ api.UpdateTemplateRequest) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}
