package api

// LetterRequest is the body of POST /internal/letters/pdf: a cover letter to render as a PDF attachment.
// Without resume_id the letter has no sender block.
type LetterRequest struct {
	ResumeID    string `json:"resume_id" binding:"omitempty,uuid"`
	Language    string `json:"language" binding:"required,oneof=fr en ar"`
	Subject     string `json:"subject" binding:"max=200"`
	Body        string `json:"body" binding:"required,max=10000"`
	SenderEmail string `json:"sender_email" binding:"omitempty,email,max=320"`
}
