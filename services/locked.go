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

const (
	errResumeLocked  = "subscribe to edit your other CVs"
	errEditNeedsPlan = "editing your CV is part of the subscription"
	// Reasons a CV is read-only.
	LockedBySubscription = "subscription"
	LockedByLimit        = "limit"
)

// lockedSet returns the user's read-only CVs with the reason, or nil when none is. Nothing is deleted. A plan with
// cv_edit = 0 (no subscription) makes every CV read-only. Otherwise, beyond the plan's CV limit, the first ones in
// EditableOrder (the chosen CV, then the last modified) stay editable, at least one; the others are read-only
// until the plan allows them again. When the plan cannot be read, nothing is locked: a student keeps editing while
// payments-api is down.
func (s *resumeService) lockedSet(ctx context.Context, userID string) (map[string]string, *models.ApiError) {
	if s.quotas == nil {
		return nil, nil
	}
	q, err := s.quotas.Quotas(ctx, userID)
	if err != nil {
		utils.Logger.Warn("quotas for the CV lock, nothing locked", zap.Error(err))
		return nil, nil
	}
	edit, hasEdit := q.Limits[core.QuotaCVEdit]
	limit, hasLimit := q.Limits[core.QuotaResumes]
	if !(hasEdit && edit <= 0) && !hasLimit {
		return nil, nil
	}
	ids, err := s.repo.EditableOrder(ctx, userID)
	if err != nil {
		return nil, repoError(err, "resume not found")
	}
	if hasEdit && edit <= 0 {
		locked := make(map[string]string, len(ids))
		for _, id := range ids {
			locked[id] = LockedBySubscription
		}
		return locked, nil
	}
	limit = max(limit, 1)
	if len(ids) <= limit {
		return nil, nil
	}
	locked := make(map[string]string, len(ids)-limit)
	for _, id := range ids[limit:] {
		locked[id] = LockedByLimit
	}
	return locked, nil
}

// canEdit says whether the plan lets the student edit CVs at all (false for cv_edit = 0). An unreadable plan
// lets them.
func (s *resumeService) canEdit(ctx context.Context, userID string) bool {
	if s.quotas == nil {
		return true
	}
	q, err := s.quotas.Quotas(ctx, userID)
	if err != nil {
		return true
	}
	edit, ok := q.Limits[core.QuotaCVEdit]
	return !ok || edit > 0
}

// checkEditable refuses a change to a locked CV with 402.
func (s *resumeService) checkEditable(ctx context.Context, userID, id string) *models.ApiError {
	locked, apiErr := s.lockedSet(ctx, userID)
	if apiErr != nil {
		return apiErr
	}
	switch locked[id] {
	case LockedBySubscription:
		return newError(http.StatusPaymentRequired, errEditNeedsPlan)
	case LockedByLimit:
		return newError(http.StatusPaymentRequired, errResumeLocked)
	}
	return nil
}

// ChooseEditable makes this CV the one the student keeps editing beyond the plan's CV limit. Without the right to
// edit at all, there is nothing to choose (402).
func (s *resumeService) ChooseEditable(ctx context.Context, userID, id string) (*api.ResumeResponse, *models.ApiError) {
	if !isUUID(id) {
		return nil, newError(http.StatusBadRequest, "invalid resume id")
	}
	if !s.canEdit(ctx, userID) {
		return nil, newError(http.StatusPaymentRequired, errEditNeedsPlan)
	}
	if err := s.repo.Pin(ctx, userID, id); err != nil {
		return nil, repoError(err, "resume not found")
	}
	return s.GetResume(ctx, userID, id)
}
