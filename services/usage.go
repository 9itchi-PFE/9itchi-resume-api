package services

import (
	"context"
	"net/http"
	"strings"
	"time"

	"9itchi-resume-api/core"
	"9itchi-resume-api/models"
	"9itchi-resume-api/models/api"
	"9itchi-resume-api/repositories"
	"9itchi-resume-api/utils"

	"go.uber.org/zap"
)

// UsageService tells the team how much of the plan a student used here. Handlers stay thin.
type UsageService interface {
	// GET /admin/users/:id/usage: CVs, downloads, AI rewrites
	UserUsage(ctx context.Context, id string) (*api.UserUsageResponse, *models.ApiError)
}

// UsageDeps are the collaborators of the usage service (all mockable). Each count is the one the limit is
// enforced on (checkResumeQuota, checkDownloadQuota, rewriteService.usage).
type UsageDeps struct {
	Quotas   core.QuotaSource
	Resumes  repositories.ResumeRepository
	Files    repositories.FileRepository
	Rewrites repositories.RewriteRepository
	Now      func() time.Time
}

type usageService struct {
	UsageDeps
}

func NewUsageService(d UsageDeps) UsageService {
	if d.Now == nil {
		d.Now = time.Now
	}
	return &usageService{d}
}

// usageItem is the plan's limit for key (nil: none, absent or negative) next to what was used.
func usageItem(limits map[string]int, key string, used int) api.UsageItem {
	item := api.UsageItem{Used: used}
	if limit, ok := limits[key]; ok && limit >= 0 {
		item.Limit = &limit
	}
	return item
}

func (s *usageService) UserUsage(ctx context.Context, id string) (*api.UserUsageResponse, *models.ApiError) {
	if !isUUID(id) {
		return nil, newError(http.StatusBadRequest, "invalid user id")
	}
	id = strings.ToLower(id)
	q, err := s.Quotas.Quotas(ctx, id)
	if err != nil {
		utils.Logger.Error("quotas", zap.Error(err))
		return nil, newError(http.StatusServiceUnavailable, errQuotaUnavailable)
	}
	resumes, err := s.Resumes.CountByUser(ctx, id)
	if err != nil {
		return nil, repoError(err, "not found")
	}
	downloads, err := s.Files.CountExportsByUser(ctx, id, q.PeriodStart)
	if err != nil {
		return nil, repoError(err, "not found")
	}
	now := s.Now().UTC()
	rewrites, err := s.Rewrites.CountSince(ctx, id, time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		return nil, repoError(err, "not found")
	}
	return &api.UserUsageResponse{
		Resumes:    usageItem(q.Limits, core.QuotaResumes, resumes),
		Downloads:  usageItem(q.Limits, core.QuotaCVDownloads, downloads),
		AIRewrites: usageItem(q.Limits, core.QuotaAIRewrites, rewrites),
	}, nil
}
