package db

import "time"

// AIRewrite maps table ai_rewrites: what the AI proposed for one line or the summary of a student's CV.
type AIRewrite struct {
	ID          string
	UserID      string
	ResumeID    string
	InputHash   string
	Suggestions []string
	CreatedAt   time.Time
}
