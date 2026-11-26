package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"9itchi-resume-api/core"
	"9itchi-resume-api/models"
	"9itchi-resume-api/models/api"
	"9itchi-resume-api/models/db"
	"9itchi-resume-api/repositories"
	"9itchi-resume-api/utils"

	"go.uber.org/zap"
)

const (
	errRewriteNeedsPlan = "the AI rewrite is part of the subscription"
	errRewriteLimit     = "monthly limit of AI rewrites of your plan reached"
)

// RewriteService asks the AI for a better version of one line, or of the summary, of a student's CV.
type RewriteService interface {
	// POST /resumes/:id/rewrite
	Rewrite(ctx context.Context, userID, resumeID string, req api.RewriteRequest) (*api.RewriteResponse, *models.ApiError)
}

type rewriteService struct {
	repo    repositories.RewriteRepository
	resumes repositories.ResumeRepository
	ai      core.Rewriter
	quotas  core.QuotaSource
	now     func() time.Time
}

func NewRewriteService(repo repositories.RewriteRepository, resumes repositories.ResumeRepository, ai core.Rewriter, quotas core.QuotaSource) RewriteService {
	return &rewriteService{repo: repo, resumes: resumes, ai: ai, quotas: quotas, now: time.Now}
}

// Rewrite gives back what the AI already proposed for the same text without asking it again nor counting it.
// Otherwise the plan must allow one more this month (402), and the AI is asked. Only the student's own CV.
func (s *rewriteService) Rewrite(ctx context.Context, userID, resumeID string, req api.RewriteRequest) (*api.RewriteResponse, *models.ApiError) {
	if !isUUID(resumeID) {
		return nil, newError(http.StatusBadRequest, "invalid resume id")
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		return nil, newError(http.StatusBadRequest, "text is required")
	}
	if res, err := s.resumes.Get(ctx, userID, resumeID); err != nil || res == nil {
		if err == nil || errors.Is(err, repositories.ErrNotFound) {
			return nil, newError(http.StatusNotFound, "resume not found")
		}
		return nil, repoError(err, "resume not found")
	}
	language := req.Language
	if language == "" {
		language = "fr"
	}
	h := sha256.New()
	for _, p := range []string{req.Section, text, language, strings.TrimSpace(req.JobTitle)} {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	hash := hex.EncodeToString(h.Sum(nil))

	found, err := s.repo.Find(ctx, userID, hash)
	switch {
	case err == nil:
		return s.respond(ctx, userID, found.Suggestions, found.Warnings)
	case !errors.Is(err, repositories.ErrNotFound):
		return nil, repoError(err, "rewrite not found")
	}
	limit, used, apiErr := s.usage(ctx, userID)
	if apiErr != nil {
		return nil, apiErr
	}
	switch {
	case limit != nil && *limit == 0:
		return nil, newError(http.StatusPaymentRequired, errRewriteNeedsPlan)
	case limit != nil && used >= *limit:
		return nil, newError(http.StatusPaymentRequired, errRewriteLimit)
	}
	suggestions, warnings, err := s.ai.Rewrite(ctx, core.RewriteInput{Section: req.Section, Text: text, Language: language, JobTitle: strings.TrimSpace(req.JobTitle)})
	if err != nil {
		utils.Logger.Warn("ai rewrite failed", zap.Error(err))
		return nil, newError(http.StatusBadGateway, "the AI is unavailable, try again")
	}
	if len(suggestions) == 0 {
		return nil, newError(http.StatusBadGateway, "the AI is unavailable, try again")
	}
	saved, err := s.repo.Save(ctx, &db.AIRewrite{UserID: userID, ResumeID: resumeID, InputHash: hash, Suggestions: suggestions, Warnings: warnings})
	if err != nil {
		return nil, repoError(err, "rewrite not found")
	}
	return s.respond(ctx, userID, saved.Suggestions, saved.Warnings)
}

// usage is the plan's monthly limit of rewrites and how many were made this calendar month (UTC); nil: no limit.
func (s *rewriteService) usage(ctx context.Context, userID string) (*int, int, *models.ApiError) {
	if s.quotas == nil {
		return nil, 0, nil
	}
	q, err := s.quotas.Quotas(ctx, userID)
	if err != nil {
		utils.Logger.Error("quotas", zap.Error(err))
		return nil, 0, newError(http.StatusServiceUnavailable, errQuotaUnavailable)
	}
	limit, ok := q.Limits[core.QuotaAIRewrites]
	if !ok || limit < 0 {
		return nil, 0, nil
	}
	now := s.now().UTC()
	n, err := s.repo.CountSince(ctx, userID, time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		return nil, 0, repoError(err, "rewrite not found")
	}
	return &limit, n, nil
}

func (s *rewriteService) respond(ctx context.Context, userID string, suggestions, warnings []string) (*api.RewriteResponse, *models.ApiError) {
	limit, used, apiErr := s.usage(ctx, userID)
	if apiErr != nil {
		return nil, apiErr
	}
	if suggestions == nil {
		suggestions = []string{}
	}
	if warnings == nil {
		warnings = []string{}
	}
	return &api.RewriteResponse{Suggestions: suggestions, Warnings: warnings, Limit: limit, Used: used}, nil
}
