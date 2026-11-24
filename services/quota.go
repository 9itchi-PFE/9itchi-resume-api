package services

import (
	"context"
	"net/http"
	"os"
	"strings"
	"unicode"

	"9itchi-resume-api/core"
	"9itchi-resume-api/models"
	"9itchi-resume-api/utils"

	"go.uber.org/zap"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

const (
	errQuotaUnavailable   = "subscription check unavailable, try again"
	errResumeLimit        = "subscription required for more than one CV"
	errDownloadLimit      = "subscription required to download your CV again"
	errWordForSubscribers = "the Word file is for subscribers"
)

// plan returns the user's quotas, or nil when no quota source is configured (no limits).
func (s *resumeService) plan(ctx context.Context, userID string) (*core.Quotas, *models.ApiError) {
	if s.quotas == nil {
		return nil, nil
	}
	q, err := s.quotas.Quotas(ctx, userID)
	if err != nil {
		utils.Logger.Error("quotas", zap.Error(err))
		return nil, newError(http.StatusServiceUnavailable, errQuotaUnavailable)
	}
	return q, nil
}

// checkResumeQuota refuses a new CV (create or upload) beyond the plan's "resumes" limit with 402.
// Versions of an existing CV are never limited.
func (s *resumeService) checkResumeQuota(ctx context.Context, userID string) (*core.Quotas, *models.ApiError) {
	q, apiErr := s.plan(ctx, userID)
	if apiErr != nil || q == nil {
		return q, apiErr
	}
	limit, ok := q.Limits[core.QuotaResumes]
	if !ok {
		return q, nil
	}
	n, err := s.repo.CountByUser(ctx, userID)
	if err != nil {
		return nil, repoError(err, "resume not found")
	}
	if n >= limit {
		return nil, newError(http.StatusPaymentRequired, errResumeLimit)
	}
	return q, nil
}

// checkDownloadQuota enforces the plan's "cv_downloads" limit (402 beyond it) and says whether the
// export carries the 9itchi mark (free plan).
func (s *resumeService) checkDownloadQuota(ctx context.Context, userID string) (branded bool, apiErr *models.ApiError) {
	if os.Getenv("UNLIMITED_DOWNLOADS") == "true" { // test environments: no plan limit on exports
		return false, nil
	}
	q, apiErr := s.plan(ctx, userID)
	if apiErr != nil || q == nil {
		return false, apiErr
	}
	branded = q.PlanName == core.FreePlanName
	limit, ok := q.Limits[core.QuotaCVDownloads]
	if !ok {
		return branded, nil
	}
	n, err := s.files.CountExportsByUser(ctx, userID)
	if err != nil {
		return false, repoError(err, "resume not found")
	}
	if n >= limit {
		return false, newError(http.StatusPaymentRequired, errDownloadLimit)
	}
	return branded, nil
}

var stripAccents = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

// exportFilename is "prenom_nom" from the CV name (rule 22), falling back to the sanitized title.
func exportFilename(name, title string) string {
	ascii, _, _ := transform.String(stripAccents, strings.ToLower(strings.TrimSpace(name)))
	var b strings.Builder
	sep := false
	for _, r := range ascii {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			if sep && b.Len() > 0 {
				b.WriteByte('_')
			}
			sep = false
			b.WriteRune(r)
		default:
			sep = true
		}
	}
	if out := b.String(); out != "" {
		if len(out) > 80 {
			out = out[:80]
		}
		return out
	}
	return sanitizeFilename(title)
}
