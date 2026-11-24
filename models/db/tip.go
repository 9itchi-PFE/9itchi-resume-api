package db

// TipClaim is a student claimed for the weekly advice, with the CV it is about.
type TipClaim struct {
	UserID   string `db:"user_id"`
	ResumeID string `db:"resume_id"`
}
