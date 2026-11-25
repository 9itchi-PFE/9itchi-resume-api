package services

import (
	"context"
	"net/http"

	"9itchi-resume-api/core"
	"9itchi-resume-api/models"
	"9itchi-resume-api/models/api"
	"9itchi-resume-api/utils"

	"go.uber.org/zap"
)

const errResumeLocked = "subscribe to edit your other CVs"

// lockedSet returns the user's CVs that are beyond the plan's CV limit, or nil when none is. Nothing is deleted:
// the first ones in EditableOrder (the chosen CV, then the last modified) stay editable, at least one; the
// others are read-only until the plan allows them again. When the plan cannot be read, nothing is locked: a
// student keeps editing while payments-api is down.
func (s *resumeService) lockedSet(ctx context.Context, userID string) (map[string]bool, *models.ApiError) {
	if s.quotas == nil {
		return nil, nil
	}
	q, err := s.quotas.Quotas(ctx, userID)
	if err != nil {
		utils.Logger.Warn("quotas for the CV lock, nothing locked", zap.Error(err))
		return nil, nil
	}
	limit, ok := q.Limits[core.QuotaResumes]
	if !ok {
		return nil, nil
	}
	limit = max(limit, 1)
	ids, err := s.repo.EditableOrder(ctx, userID)
	if err != nil {
		return nil, repoError(err, "resume not found")
	}
	if len(ids) <= limit {
		return nil, nil
	}
	locked := make(map[string]bool, len(ids)-limit)
	for _, id := range ids[limit:] {
		locked[id] = true
	}
	return locked, nil
}

// checkEditable refuses a change to a locked CV with 402.
func (s *resumeService) checkEditable(ctx context.Context, userID, id string) *models.ApiError {
	locked, apiErr := s.lockedSet(ctx, userID)
	if apiErr != nil {
		return apiErr
	}
	if locked[id] {
		return newError(http.StatusPaymentRequired, errResumeLocked)
	}
	return nil
}

// ChooseEditable makes this CV the one the student keeps editing beyond the plan's CV limit.
func (s *resumeService) ChooseEditable(ctx context.Context, userID, id string) (*api.ResumeResponse, *models.ApiError) {
	if !isUUID(id) {
		return nil, newError(http.StatusBadRequest, "invalid resume id")
	}
	if err := s.repo.Pin(ctx, userID, id); err != nil {
		return nil, repoError(err, "resume not found")
	}
	return s.GetResume(ctx, userID, id)
}
