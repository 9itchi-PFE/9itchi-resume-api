package services

import (
	"context"
	"net/http"

	"9itchi-resume-api/models"
	"9itchi-resume-api/models/api"
	"9itchi-resume-api/models/mapping"
	"9itchi-resume-api/repositories"
)

// TemplateService holds all business logic for template. Handlers stay thin.
type TemplateService interface {
	// GET /templates: active template gallery
	ListTemplates(ctx context.Context, userID string) (*api.ListTemplatesResponse, *models.ApiError)
	// GET /templates/:id: template detail
	GetTemplate(ctx context.Context, userID string, id string) (*api.TemplateResponse, *models.ApiError)
	// GET /admin/templates: every template, published or not, with how many CVs use it
	ListAllTemplates(ctx context.Context) (*api.AdminTemplatesResponse, *models.ApiError)
	// POST /admin/templates: add template
	CreateTemplate(ctx context.Context, userID string, req api.CreateTemplateRequest) (*api.TemplateResponse, *models.ApiError)
	// PUT /admin/templates/:id: edit or deactivate template
	UpdateTemplate(ctx context.Context, userID string, id string, req api.UpdateTemplateRequest) (*api.TemplateResponse, *models.ApiError)
}

type templateService struct {
	repo repositories.TemplateRepository
}

func NewTemplateService(repo repositories.TemplateRepository) TemplateService {
	return &templateService{repo: repo}
}

func (s *templateService) ListTemplates(ctx context.Context, _ string) (*api.ListTemplatesResponse, *models.ApiError) {
	items, err := s.repo.ListActive(ctx)
	if err != nil {
		return nil, repoError(err, "template not found")
	}
	out := &api.ListTemplatesResponse{Items: make([]api.TemplateResponse, 0, len(items))}
	for i := range items {
		out.Items = append(out.Items, mapping.TemplateToResponse(&items[i]))
	}
	return out, nil
}

func (s *templateService) ListAllTemplates(ctx context.Context) (*api.AdminTemplatesResponse, *models.ApiError) {
	items, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, repoError(err, "template not found")
	}
	out := &api.AdminTemplatesResponse{Items: make([]api.AdminTemplateResponse, 0, len(items))}
	for i := range items {
		out.Items = append(out.Items, api.AdminTemplateResponse{TemplateResponse: mapping.TemplateToResponse(&items[i].Template), Resumes: items[i].Resumes})
	}
	return out, nil
}

func (s *templateService) GetTemplate(ctx context.Context, _ string, id string) (*api.TemplateResponse, *models.ApiError) {
	if !isUUID(id) {
		return nil, newError(http.StatusBadRequest, "invalid template id")
	}
	t, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, repoError(err, "template not found")
	}
	resp := mapping.TemplateToResponse(t)
	return &resp, nil
}

func (s *templateService) CreateTemplate(ctx context.Context, _ string, req api.CreateTemplateRequest) (*api.TemplateResponse, *models.ApiError) {
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	t, err := s.repo.Create(ctx, req.Name, req.Layout, req.PreviewURL, active)
	if err != nil {
		return nil, repoError(err, "template not found")
	}
	resp := mapping.TemplateToResponse(t)
	return &resp, nil
}

func (s *templateService) UpdateTemplate(ctx context.Context, _ string, id string, req api.UpdateTemplateRequest) (*api.TemplateResponse, *models.ApiError) {
	if !isUUID(id) {
		return nil, newError(http.StatusBadRequest, "invalid template id")
	}
	// Students must always have a template to choose: the last published one cannot be withdrawn.
	if req.Active != nil && !*req.Active {
		others, err := s.repo.CountOtherActive(ctx, id)
		if err != nil {
			return nil, repoError(err, "template not found")
		}
		if others == 0 {
			return nil, newError(http.StatusConflict, "the last published template cannot be withdrawn")
		}
	}
	t, err := s.repo.Update(ctx, id, repositories.TemplatePatch{
		Name: req.Name, Layout: req.Layout, PreviewURL: req.PreviewURL, Active: req.Active,
	})
	if err != nil {
		return nil, repoError(err, "template not found")
	}
	resp := mapping.TemplateToResponse(t)
	return &resp, nil
}
