package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"9itchi-resume-api/core"
	"9itchi-resume-api/models/api"
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
	started := time.Now()
	fail := func(step string, err error) {
		utils.Logger.Warn("cv reading failed", zap.String("step", step), zap.String("resume_id", resumeID), zap.Error(err))
		if err := s.repo.SetParseStatus(ctx, resumeID, db.ParseFailed); err != nil {
			utils.Logger.Error("cv reading: mark failed", zap.Error(err))
		}
	}
	var parsed *core.ParsedResume
	var err error
	var text string
	if format == "jpeg" || format == "png" {
		// A photo of a CV has no text layer: ai-api reads the picture itself.
		parsed, err = s.parser.ParseImage(ctx, "image/"+format, data)
	} else {
		if text, err = core.ExtractText(format, data); err != nil {
			fail("extract", err)
			return
		}
		parsed, err = s.parser.Parse(ctx, text)
	}
	if err != nil {
		fail("parse", err)
		return
	}
	content := withDefaultStyle(parsed.Content)
	if len(content.SectionOrderKeys) == 0 {
		content.SectionOrderKeys = core.InferSectionOrder(text, content)
	}
	var photo []byte
	if format != "jpeg" && format != "png" {
		photo = core.ExtractPhoto(format, data)
	}
	if photo == nil && content.Style.ShowPhoto == nil {
		// The uploaded CV shows no photo: keep it that way instead of an initials placeholder.
		hide := false
		content.Style.ShowPhoto = &hide
	}
	raw, apiErr := encodeContent(content)
	if apiErr != nil {
		fail("encode", apiErr.Error)
		return
	}
	lang := parsed.Language
	if lang != "fr" && lang != "en" && lang != "ar" {
		lang = ""
	}
	removed := []string{}
	for _, r := range parsed.Removed {
		switch r {
		case "age", "birth_date", "nationality", "marital_status", "full_address":
			removed = append(removed, r)
		}
	}
	report, _ := json.Marshal(api.ParseReport{
		Pages:     core.PageCount(data, format),
		Seconds:   int(time.Since(started).Round(time.Second) / time.Second),
		Rewritten: max(parsed.Rewritten, 0),
		Removed:   removed,
	})
	applied, err := s.repo.ApplyParsed(ctx, resumeID, raw, lang, report)
	if err != nil {
		fail("store", err)
		return
	}
	if !applied {
		utils.Logger.Info("cv reading discarded: edited meanwhile", zap.String("resume_id", resumeID))
		return
	}
	s.importPhoto(ctx, resumeID, photo)
}

// importPhoto keeps the picture embedded in the uploaded CV as the CV photo, unless the user already set one.
func (s *resumeService) importPhoto(ctx context.Context, resumeID string, photo []byte) {
	if photo == nil {
		return
	}
	if has, err := s.files.HasPhoto(ctx, resumeID); err != nil || has {
		return
	}
	sum := sha256.Sum256(photo)
	if _, err := s.files.Create(ctx, &db.ResumeFile{
		ResumeID: resumeID, Kind: "photo", Format: "jpeg", Filename: "photo.jpeg", ContentType: "image/jpeg",
		SizeBytes: int64(len(photo)), SHA256: hex.EncodeToString(sum[:]), Data: photo,
	}); err != nil {
		utils.Logger.Warn("cv reading: store photo", zap.String("resume_id", resumeID), zap.Error(err))
	}
}
