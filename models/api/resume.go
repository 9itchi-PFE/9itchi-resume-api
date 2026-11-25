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
//	{"contact":{"name","headline","email","phone","location","links":["https://..."]},
//	 "summary":"...",
//	 "experience":[{"company","title","kind","location","start","end","description":["bullet"]}],
//	 "education":[{"school","degree","location","start","end"}],
//	 "projects":[{"name","tech","year","description":["bullet"]}],
//	 "skills":["Go"], "skill_groups":[{"label":"Langages","items":["C++"]}],
//	 "languages":["Arabe: Langue maternelle"],
//	 "certifications":[{"name","issuer","year"}],
//	 "extra":[{"title":"Clubs","items":["..."]}]}
//
// Dates (start/end) are free text such as "2021-03" or "Present". A language may carry its level after a colon.
// skill_groups, when present, is what the templates show; skills stays the flat list. The new fields are optional,
// so older documents still parse. The whole document is capped at MaxContentBytes.
type ResumeContent struct {
	Contact        Contact         `json:"contact"`
	Summary        string          `json:"summary" binding:"max=5000"`
	Experience     []Experience    `json:"experience" binding:"max=50,dive"`
	Education      []Education     `json:"education" binding:"max=30,dive"`
	Skills         []string        `json:"skills" binding:"max=100,dive,max=100"`
	Languages      []string        `json:"languages" binding:"max=30,dive,max=100"`
	Certifications []Certification `json:"certifications" binding:"max=30,dive"`
	Projects       []Project       `json:"projects,omitempty" binding:"max=30,dive"`
	SkillGroups    []SkillGroup    `json:"skill_groups,omitempty" binding:"max=10,dive"`
	Extra          []ExtraSection  `json:"extra,omitempty" binding:"max=10,dive"`
	// SectionOrderKeys is the order the user dragged the sections into; the contact block always stays on top.
	SectionOrderKeys []string `json:"section_order,omitempty" binding:"max=20,dive,max=20"`
	// HiddenSections are kept in the content but left out of the preview and every export.
	HiddenSections []string `json:"hidden_sections,omitempty" binding:"max=20,dive,max=20"`
	Style          *CVStyle `json:"style,omitempty"`
}

// AllSkills is the flat skill list followed by the grouped items not already in it.
func (c ResumeContent) AllSkills() []string {
	out := append([]string(nil), c.Skills...)
	seen := map[string]bool{}
	for _, s := range c.Skills {
		seen[s] = true
	}
	for _, g := range c.SkillGroups {
		for _, s := range g.Items {
			if !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	return out
}

type Project struct {
	Name        string   `json:"name" binding:"max=200"`
	Tech        string   `json:"tech" binding:"max=200"`
	HideTech    bool     `json:"hide_tech,omitempty"`
	URL         string   `json:"url,omitempty" binding:"max=300"`
	Hidden      bool     `json:"hidden,omitempty"`
	Year        string   `json:"year" binding:"max=30"`
	Start       string   `json:"start,omitempty" binding:"max=30"`
	End         string   `json:"end,omitempty" binding:"max=30"`
	Location    string   `json:"location,omitempty" binding:"max=200"`
	Description []string `json:"description" binding:"max=30,dive,max=1000"`
}

type SkillGroup struct {
	Label string   `json:"label" binding:"max=100"`
	Items []string `json:"items" binding:"max=50,dive,max=100"`
}

// ExtraSection is a section added with "+ Ajouter une section" (clubs, prix, bénévolat...).
type ExtraSection struct {
	Title string   `json:"title" binding:"max=100"`
	Items []string `json:"items" binding:"max=30,dive,max=1000"`
}

type Certification struct {
	Name   string `json:"name" binding:"max=200"`
	Issuer string `json:"issuer" binding:"max=200"`
	URL    string `json:"url,omitempty" binding:"max=300"`
	Hidden bool   `json:"hidden,omitempty"`
	Year   string `json:"year" binding:"max=30"`
}

type Contact struct {
	Name     string   `json:"name" binding:"max=200"`
	Headline string   `json:"headline,omitempty" binding:"max=200"`
	Email    string   `json:"email" binding:"max=200"`
	Phone    string   `json:"phone" binding:"max=50"`
	Location string   `json:"location" binding:"max=200"`
	Links    []string `json:"links" binding:"max=20,dive,max=300"`
}

type Experience struct {
	Company     string   `json:"company" binding:"max=200"`
	Title       string   `json:"title" binding:"max=200"`
	Kind        string   `json:"kind,omitempty" binding:"max=100"`
	URL         string   `json:"url,omitempty" binding:"max=300"`
	Hidden      bool     `json:"hidden,omitempty"`
	Location    string   `json:"location,omitempty" binding:"max=200"`
	Start       string   `json:"start" binding:"max=30"`
	End         string   `json:"end" binding:"max=30"`
	Description []string `json:"description" binding:"max=30,dive,max=1000"`
}

type Education struct {
	School   string `json:"school" binding:"max=200"`
	Degree   string `json:"degree" binding:"max=200"`
	URL      string `json:"url,omitempty" binding:"max=300"`
	Hidden   bool   `json:"hidden,omitempty"`
	Location string `json:"location,omitempty" binding:"max=200"`
	Start    string `json:"start" binding:"max=30"`
	End      string `json:"end" binding:"max=30"`
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
	ParseStatus string  `json:"parse_status"`
	Language    *string `json:"language"`
	// ParseReport says what the reading changed (screen 1.2c); null until an upload is read.
	ParseReport *ParseReport `json:"parse_report"`
	// HasPhoto is true when a photo was uploaded (GET /resumes/:id/photo).
	HasPhoto bool `json:"has_photo"`
	// Locked is true when the CV is beyond the plan's CV limit: it can be read and downloaded, not changed.
	Locked    bool          `json:"locked"`
	Content   ResumeContent `json:"content"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// ResumeListItem is a resume without content.
type ResumeListItem struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	TemplateID *string `json:"template_id"`
	Version    int     `json:"version"`
	// Locked: see ResumeResponse.
	Locked    bool      `json:"locked"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
	Label     string    `json:"label"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateVersionRequest is the optional body of POST /resumes/:id/versions: a short key for the history tab
// (bullet_rewritten, template_changed, section_added, manual...), translated by the client.
type CreateVersionRequest struct {
	Label string `json:"label" binding:"omitempty,max=100,printascii"`
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

// ATSRuleResult is one of the 24 rules: status ok|fix|tip, or locked when the student's plan does not show its
// group; Where/Was/Now are set on fix (Now also on file_name).
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
	// LockedGroups are the groups the student's plan does not show (their rules are "locked"); Passed, Total and
	// ToFix still count the whole CV.
	LockedGroups []string `json:"locked_groups"`
}

// ParseReport describes the reading of an uploaded CV.
type ParseReport struct {
	// Pages of the uploaded file (0 for DOCX) and how long the reading took.
	Pages   int `json:"pages"`
	Seconds int `json:"seconds"`
	// Rewritten lines now start with a past-tense verb; Removed lists personal details left out
	// (age, birth_date, nationality, marital_status, full_address).
	Rewritten int      `json:"rewritten"`
	Removed   []string `json:"removed"`
}

// VersionContentResponse is one saved version with its content.
type VersionContentResponse struct {
	VersionResponse
	Content ResumeContent `json:"content"`
}

// DownloadsResponse says how many times the user downloaded a CV in the current period of the subscription (ever,
// without one) and how many the plan allows: Limit is nil when the plan sets none, 0 when it allows no download.
type DownloadsResponse struct {
	Used  int  `json:"used"`
	Limit *int `json:"limit"`
}

// DuplicateResumeRequest is the body of POST /resumes/:id/duplicate.
type DuplicateResumeRequest struct {
	Title string `json:"title" binding:"required,max=200"`
}

// TipsResponse counts the CV tips raised by one call, and those that could not be raised (they are tried again at
// the next call).
type TipsResponse struct {
	Sent   int `json:"sent"`
	Failed int `json:"failed"`
}

// UserDownloadsResponse is what the team sees of a student's CV downloads: how many, and when the last one was.
// It is what tells whether a payment was used (the refund rule); it says nothing of which CV.
type UserDownloadsResponse struct {
	Count  int        `json:"count"`
	LastAt *time.Time `json:"last_at"`
}

// UserCVResponse is where a student's CV stands, for the team: counts only, nothing of what the CV says.
type UserCVResponse struct {
	// ResumeID and Title are the CV the student changed last; Resumes is how many they have.
	ResumeID string `json:"resume_id"`
	Title    string `json:"title"`
	Resumes  int    `json:"resumes"`
	Version  int    `json:"version"`
	// Passed of Total rules are met; ToFix are left to correct.
	Passed    int       `json:"passed"`
	Total     int       `json:"total"`
	ToFix     int       `json:"to_fix"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RewriteRequest is the body of POST /resumes/:id/rewrite: one line of the CV, or its summary, to rewrite.
type RewriteRequest struct {
	Section  string `json:"section" binding:"required,oneof=bullet summary"`
	Text     string `json:"text" binding:"required,max=2000"`
	Language string `json:"language" binding:"omitempty,oneof=fr en ar"`
	JobTitle string `json:"job_title" binding:"max=200"`
}

// RewriteResponse is what the AI proposes, with what is left of the plan's rewrites this month (Limit nil: no limit).
type RewriteResponse struct {
	Suggestions []string `json:"suggestions"`
	Limit       *int     `json:"limit"`
	Used        int      `json:"used"`
}
