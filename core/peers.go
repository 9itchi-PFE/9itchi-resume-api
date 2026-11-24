package core

import (
	"context"
	"errors"

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
}

// Quotas are the limits of the user's current plan (payments-api). A key that is absent means no limit.
type Quotas struct {
	PlanName string
	Limits   map[string]int
}

// FreePlanName is payments-api's fallback plan; exports on it carry the 9itchi mark.
const FreePlanName = "Free"

// Quota keys shared with payments-api.
const (
	QuotaResumes     = "resumes"
	QuotaCVDownloads = "cv_downloads"
)

// QuotaSource reads the user's plan quotas.
type QuotaSource interface {
	Quotas(ctx context.Context, userID string) (*Quotas, error)
}

// ErrPeerUnavailable is returned by peer clients when the peer cannot answer.
var ErrPeerUnavailable = errors.New("peer service unavailable")
