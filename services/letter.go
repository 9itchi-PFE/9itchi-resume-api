package services

import (
	"context"
	"errors"
	"net/http"
	"time"

	"9itchi-resume-api/core"
	"9itchi-resume-api/models"
	"9itchi-resume-api/models/api"
	"9itchi-resume-api/models/mapping"
	"9itchi-resume-api/repositories"
	"9itchi-resume-api/utils"

	"go.uber.org/zap"
)

// tunis is Tunisia's time (UTC+1, no summer time): the date printed on a letter.
var tunis = time.FixedZone("Africa/Tunis", 3600)

// LetterService renders cover letters for email-api.
type LetterService interface {
	// POST /internal/letters/pdf (internal): the letter as a PDF, unstamped and not counted
	LetterPDF(ctx context.Context, req api.LetterRequest) ([]byte, *models.ApiError)
}

type letterService struct {
	resumes repositories.ResumeRepository
	now     func() time.Time
}

func NewLetterService(resumes repositories.ResumeRepository) LetterService {
	return &letterService{resumes: resumes, now: time.Now}
}

// LetterPDF lays out the letter in the CV's font and accent with its contact as the sender block. A CV that no
// longer exists only drops the sender block: the letter still goes out.
func (s *letterService) LetterPDF(ctx context.Context, req api.LetterRequest) ([]byte, *models.ApiError) {
	switch req.Language {
	case "fr", "en", "ar":
	default:
		return nil, newError(http.StatusBadRequest, "invalid language")
	}
	in := core.LetterInput{
		Language: req.Language, Subject: req.Subject, Body: req.Body, SenderEmail: req.SenderEmail,
		Now: s.now().In(tunis),
	}
	if req.ResumeID != "" {
		if !isUUID(req.ResumeID) {
			return nil, newError(http.StatusBadRequest, "invalid resume id")
		}
		res, err := s.resumes.GetByID(ctx, req.ResumeID)
		switch {
		case errors.Is(err, repositories.ErrNotFound):
		case err != nil:
			return nil, repoError(err, "resume not found")
		case res != nil:
			content := mapping.ParseContent(res.Content)
			in.Contact, in.Style = &content.Contact, content.Style
		}
	}
	data, err := core.RenderLetter(ctx, in)
	if err != nil {
		utils.Logger.Error("letter render failed", zap.Error(err))
		return nil, newError(http.StatusInternalServerError, "letter failed")
	}
	return data, nil
}
