package core

import (
	"context"
	"errors"
	"time"

	"9itchi-resume-api/models/api"
)

// ParsedResume is what the AI reader returns for a CV's text.
type ParsedResume struct {
	Content  api.ResumeContent
	Language string
	// Rewritten is how many lines the AI turned into past-tense action lines; Removed lists the personal details
	// it left out (age, birth_date, nationality, marital_status, full_address).
	Rewritten int
	Removed   []string
}

// ResumeParser turns CV text into structured content (ai-api). Implementations never log the text.
type ResumeParser interface {
	Parse(ctx context.Context, text string) (*ParsedResume, error)
	// ParseImage reads a photo or scan of a CV (mediaType image/jpeg or image/png).
	ParseImage(ctx context.Context, mediaType string, data []byte) (*ParsedResume, error)
}

// Quotas are the limits of the user's current plan (payments-api). A key that is absent means no limit.
type Quotas struct {
	PlanName string
	Limits   map[string]int
	// PeriodStart is when the current period of the subscription began; nil without a subscription. Limits "per
	// period" (downloads) count from there.
	PeriodStart *time.Time
}

// FreePlanName is payments-api's plan of a student with no subscription: no download, and only a part of the
// diagnosis. An export on it, should its limits ever allow one, carries the 9itchi mark.
const FreePlanName = "Free"

// Quota keys shared with payments-api.
const (
	QuotaResumes = "resumes"
	// QuotaCVDownloads is how many times a CV can be downloaded in a period of the subscription (0: never).
	QuotaCVDownloads = "cv_downloads"
	// QuotaATSPrefix + a group of the diagnosis (ats_experience, ats_layout...) set to 0 hides that group.
	QuotaATSPrefix = "ats_"
	// QuotaAIRewrites is the monthly limit of AI rewrites of a CV line or of the summary (absent: no limit; 0:
	// none). A rewrite of the same text, given back again, is not counted.
	QuotaAIRewrites = "ai_rewrites_per_month"
	// QuotaCVEdit set to 0 makes every CV read-only: it is imported and read, not edited in 9itchi (no
	// subscription, decided 25 Nov. 2026).
	QuotaCVEdit = "cv_edit"
)

// RewriteInput is one line (or the summary) of a CV to rewrite, for an offer when JobTitle is set.
type RewriteInput struct {
	Section  string
	Text     string
	Language string
	JobTitle string
}

// Rewriter calls ai-api's internal rewrite route. Implementations never log the text.
type Rewriter interface {
	Rewrite(ctx context.Context, in RewriteInput) ([]string, error)
}

// Shows says whether the plan shows a group of the diagnosis: hidden only when its key is 0.
func (q *Quotas) Shows(group string) bool {
	if q == nil {
		return true
	}
	limit, set := q.Limits[QuotaATSPrefix+group]
	return !set || limit != 0
}

// QuotaSource reads the user's plan quotas.
type QuotaSource interface {
	Quotas(ctx context.Context, userID string) (*Quotas, error)
}

// ErrPeerUnavailable is returned by peer clients when the peer cannot answer.
var ErrPeerUnavailable = errors.New("peer service unavailable")
