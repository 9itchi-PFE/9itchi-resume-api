package api

import "time"

// Limits enforced by the resume service.
const (
	MaxTitleLen     = 200
	MaxContentBytes = 200 * 1024
	MaxUploadBytes  = 10 * 1024 * 1024
)

// ResumeContent is the documented schema of resumes.content (jsonb). Template switches never touch it.
//
//	{"contact":{"name","email","phone","location","links":["https://..."]},
//	 "summary":"...",
//	 "experience":[{"company","title","start","end","description":["bullet"]}],
//	 "education":[{"school","degree","start","end"}],
//	 "skills":["Go"], "languages":["English"],
//	 "certifications":[{"name","issuer","year"}]}
//
// Dates (start/end) are free text such as "2021-03" or "Present". The whole document is capped at MaxContentBytes.
type ResumeContent struct {
	Contact        Contact         `json:"contact"`
	Summary        string          `json:"summary" binding:"max=5000"`
	Experience     []Experience    `json:"experience" binding:"max=50,dive"`
	Education      []Education     `json:"education" binding:"max=30,dive"`
	Skills         []string        `json:"skills" binding:"max=100,dive,max=100"`
	Languages      []string        `json:"languages" binding:"max=30,dive,max=100"`
	Certifications []Certification `json:"certifications" binding:"max=30,dive"`
}

type Certification struct {
	Name   string `json:"name" binding:"max=200"`
	Issuer string `json:"issuer" binding:"max=200"`
	Year   string `json:"year" binding:"max=30"`
}

type Contact struct {
	Name     string   `json:"name" binding:"max=200"`
	Email    string   `json:"email" binding:"max=200"`
	Phone    string   `json:"phone" binding:"max=50"`
	Location string   `json:"location" binding:"max=200"`
	Links    []string `json:"links" binding:"max=20,dive,max=300"`
}

type Experience struct {
	Company     string   `json:"company" binding:"max=200"`
	Title       string   `json:"title" binding:"max=200"`
	Start       string   `json:"start" binding:"max=30"`
	End         string   `json:"end" binding:"max=30"`
	Description []string `json:"description" binding:"max=30,dive,max=1000"`
}

type Education struct {
	School string `json:"school" binding:"max=200"`
	Degree string `json:"degree" binding:"max=200"`
	Start  string `json:"start" binding:"max=30"`
	End    string `json:"end" binding:"max=30"`
}

// CreateResumeRequest is the body of POST /resumes.
type CreateResumeRequest struct {
	Title      string        `json:"title" binding:"required,max=200"`
	TemplateID *string       `json:"template_id" binding:"omitempty,uuid"`
	Content    ResumeContent `json:"content"`
}

// UpdateResumeRequest is the body of PUT /resumes/:id. Only provided fields change; changing
// template_id alone never alters content.
type UpdateResumeRequest struct {
	Title      *string        `json:"title" binding:"omitempty,min=1,max=200"`
	TemplateID *string        `json:"template_id" binding:"omitempty,uuid"`
	Content    *ResumeContent `json:"content"`
}

// ExportResumeRequest is the body of POST /resumes/:id/export.
type ExportResumeRequest struct {
	Format string `json:"format" binding:"required"`
}

type ResumeResponse struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	TemplateID *string `json:"template_id"`
	Version    int     `json:"version"`
	// ParseStatus is none|pending|done|failed while an uploaded CV is read; poll GET /resumes/:id until it leaves pending.
	ParseStatus string        `json:"parse_status"`
	Language    *string       `json:"language"`
	Content     ResumeContent `json:"content"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// ResumeListItem is a resume without content.
type ResumeListItem struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	TemplateID *string   `json:"template_id"`
	Version    int       `json:"version"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type ListResumesResponse struct {
	Items    []ResumeListItem `json:"items"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

type VersionResponse struct {
	ID        string    `json:"id"`
	VersionNo int       `json:"version_no"`
	CreatedAt time.Time `json:"created_at"`
}

type ListVersionsResponse struct {
	Items []VersionResponse `json:"items"`
}

// UploadResumeResponse is returned by POST /resumes/upload. Content stays empty until AI parsing is wired.
type UploadResumeResponse struct {
	Resume ResumeResponse `json:"resume"`
	File   FileResponse   `json:"file"`
}

// ATSCheckRequest is the optional body of POST /resumes/:id/ats-check.
type ATSCheckRequest struct {
	// TargetLanguage is the language of the company the student applies to (rule company_language).
	TargetLanguage string `json:"target_language" binding:"omitempty,oneof=fr en ar"`
}

// ATSFinding points at the text that breaks a rule.
type ATSFinding struct {
	Where string `json:"where,omitempty"`
	Was   string `json:"was,omitempty"`
	Now   string `json:"now,omitempty"`
}

// ATSRuleResult is one of the 24 rules: status ok|fix|tip; Where/Was/Now are set on fix (Now also on file_name).
type ATSRuleResult struct {
	ID     string `json:"id"`
	Group  string `json:"group"`
	Status string `json:"status"`
	ATSFinding
}

// ATSCheckResponse is the result of the 24 rules (screen 4.2).
type ATSCheckResponse struct {
	Passed int             `json:"passed"`
	Total  int             `json:"total"`
	ToFix  int             `json:"to_fix"`
	Rules  []ATSRuleResult `json:"rules"`
}
