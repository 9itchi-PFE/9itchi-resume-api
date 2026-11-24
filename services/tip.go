package services

import (
	"context"
	"time"

	"9itchi-resume-api/core"
	"9itchi-resume-api/models"
	"9itchi-resume-api/models/api"
	"9itchi-resume-api/models/db"
	"9itchi-resume-api/repositories"
	"9itchi-resume-api/utils"

	"go.uber.org/zap"
)

const (
	// tipEvery is the shortest time between two tips for one student: once a week, with room for a scheduler that
	// does not tick at the same minute every week.
	tipEvery = 7*24*time.Hour - 4*time.Hour
	// tipIdle is how long a CV must have been left alone before it is tipped: a student who is working on it does
	// not need to be told.
	tipIdle = 3 * 24 * time.Hour
	// tipBatch bounds one call (each CV is rendered to be checked); the rest is taken at the next one.
	tipBatch = 50
)

// TipService raises the weekly advice on a CV (« Paramètres → Alertes → Conseils CV »). It only raises the event:
// notification-api decides, from the student's settings, whether it becomes a notification, an email or nothing.
type TipService interface {
	// POST /internal/tips/send: tell the students whose CV still has points to correct, once a week at most
	Send(ctx context.Context) (*api.TipsResponse, *models.ApiError)
}

type tipService struct {
	tips      repositories.TipRepository
	resumes   repositories.ResumeRepository
	templates repositories.TemplateRepository
	events    core.Publisher
	now       func() time.Time
	// check runs the 24 rules on a CV; tests replace it.
	check func(ctx context.Context, res *db.Resume) api.ATSCheckResponse
}

func NewTipService(tips repositories.TipRepository, resumes repositories.ResumeRepository, templates repositories.TemplateRepository, events core.Publisher) TipService {
	s := &tipService{tips: tips, resumes: resumes, templates: templates, events: events, now: time.Now}
	s.check = func(ctx context.Context, res *db.Resume) api.ATSCheckResponse {
		return checkResume(ctx, s.templates, res, "")
	}
	return s
}

func (s *tipService) Send(ctx context.Context) (*api.TipsResponse, *models.ApiError) {
	now := s.now()
	claimed, err := s.tips.Claim(ctx, now.Add(-tipIdle), now.Add(-tipEvery), tipBatch)
	if err != nil {
		return nil, repoError(err, "resume not found")
	}
	res := &api.TipsResponse{}
	for _, c := range claimed {
		resume, err := s.resumes.GetByID(ctx, c.ResumeID)
		if err != nil {
			// The CV was deleted meanwhile, or could not be read: the student is asked about again at the next call.
			s.release(ctx, c.UserID)
			res.Failed++
			continue
		}
		check := s.check(ctx, resume)
		if check.ToFix == 0 {
			// Nothing to correct: the student is asked about again in a week.
			continue
		}
		rule := ""
		for _, r := range check.Rules {
			if r.Status == core.RuleFix {
				rule = r.ID
				break
			}
		}
		payload := map[string]any{"user_id": c.UserID, "resume_id": c.ResumeID, "count": check.ToFix, "rule": rule}
		if err := s.events.Publish(ctx, core.EventCVTips, payload); err != nil {
			// Not raised: the claim is given back so that the next call tries again.
			utils.Logger.Warn("publish event", zap.String("event", core.EventCVTips), zap.String("resume_id", c.ResumeID), zap.Error(err))
			s.release(ctx, c.UserID)
			res.Failed++
			continue
		}
		res.Sent++
	}
	return res, nil
}

func (s *tipService) release(ctx context.Context, userID string) {
	if err := s.tips.Release(ctx, userID); err != nil {
		utils.Logger.Error("release cv tip", zap.Error(err))
	}
}
