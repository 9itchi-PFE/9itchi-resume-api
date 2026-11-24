package services

import (
	"context"
	"time"

	"9itchi-resume-api/core"
	"9itchi-resume-api/models/db"
	"9itchi-resume-api/utils"

	"go.uber.org/zap"
)

// parseTimeout bounds the background reading of one uploaded CV.
const parseTimeout = 2 * time.Minute

// readUpload extracts the text of an uploaded CV, asks ai-api to structure it and stores the result,
// unless the user edited the CV meanwhile (ApplyParsed only writes while the status is still pending).
// It runs in the background with its own context; failures only flip parse_status to failed.
func (s *resumeService) readUpload(resumeID, format string, data []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), parseTimeout)
	defer cancel()
	fail := func(step string, err error) {
		utils.Logger.Warn("cv reading failed", zap.String("step", step), zap.String("resume_id", resumeID), zap.Error(err))
		if err := s.repo.SetParseStatus(ctx, resumeID, db.ParseFailed); err != nil {
			utils.Logger.Error("cv reading: mark failed", zap.Error(err))
		}
	}
	text, err := core.ExtractText(format, data)
	if err != nil {
		fail("extract", err)
		return
	}
	parsed, err := s.parser.Parse(ctx, text)
	if err != nil {
		fail("parse", err)
		return
	}
	raw, apiErr := encodeContent(parsed.Content)
	if apiErr != nil {
		fail("encode", apiErr.Error)
		return
	}
	lang := parsed.Language
	if lang != "fr" && lang != "en" && lang != "ar" {
		lang = ""
	}
	applied, err := s.repo.ApplyParsed(ctx, resumeID, raw, lang)
	if err != nil {
		fail("store", err)
		return
	}
	if !applied {
		utils.Logger.Info("cv reading discarded: edited meanwhile", zap.String("resume_id", resumeID))
	}
}
