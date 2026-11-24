package services

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"9itchi-resume-api/core"
	"9itchi-resume-api/models/api"
	"9itchi-resume-api/models/db"
	"9itchi-resume-api/repositories"
	"9itchi-resume-api/services/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tipEvents is a core.Publisher that keeps what it was given and fails for the users in failFor.
type tipEvents struct {
	sent    []map[string]any
	failFor map[string]bool
}

func (r *tipEvents) Publish(_ context.Context, event string, payload any) error {
	p, _ := payload.(map[string]any)
	if event != core.EventCVTips {
		return errors.New("unexpected event " + event)
	}
	if user, _ := p["user_id"].(string); r.failFor[user] {
		return errors.New("broker down")
	}
	r.sent = append(r.sent, p)
	return nil
}

func TestSendTips(t *testing.T) {
	now := time.Date(2026, 11, 24, 10, 0, 0, 0, time.UTC)
	const clean, gone, unlucky = "clean-user", "gone-user", "unlucky-user"
	tips, resumes, events := &mocks.TipRepo{}, &mocks.ResumeRepo{}, &tipEvents{failFor: map[string]bool{unlucky: true}}
	svc := NewTipService(tips, resumes, &mocks.TemplateRepo{}, events).(*tipService)
	svc.now = func() time.Time { return now }

	var idleBefore, tippedBefore time.Time
	var limit int
	tips.ClaimFn = func(_ context.Context, idle, tipped time.Time, l int) ([]db.TipClaim, error) {
		idleBefore, tippedBefore, limit = idle, tipped, l
		return []db.TipClaim{
			{UserID: uid, ResumeID: rid}, {UserID: clean, ResumeID: "cv-clean"},
			{UserID: gone, ResumeID: "cv-gone"}, {UserID: unlucky, ResumeID: "cv-unlucky"},
		}, nil
	}
	resumes.GetByIDFn = func(_ context.Context, id string) (*db.Resume, error) {
		if id == "cv-gone" {
			return nil, repositories.ErrNotFound
		}
		// The real check runs on this one: no phone, no summary, nothing to read.
		return &db.Resume{ID: id, Content: []byte(`{"contact":{"name":"Ines Mansour","email":"i@x.tn","location":"Sfax"}}`)}, nil
	}
	real := svc.check
	svc.check = func(ctx context.Context, res *db.Resume) api.ATSCheckResponse {
		if res.ID == "cv-clean" {
			return api.ATSCheckResponse{Total: 24, Passed: 24}
		}
		return real(ctx, res)
	}

	res, apiErr := svc.Send(context.Background())
	require.Nil(t, apiErr)
	assert.Equal(t, 1, res.Sent, "the clean CV is not told, the deleted one and the unpublished one are not counted as sent")
	assert.Equal(t, 2, res.Failed)
	assert.Equal(t, []string{gone, unlucky}, tips.Released, "what could not be raised is tried again")
	assert.Equal(t, now.Add(-3*24*time.Hour), idleBefore, "a CV being worked on is left alone")
	assert.Equal(t, now.Add(-(7*24-4)*time.Hour), tippedBefore, "once a week")
	assert.Equal(t, tipBatch, limit)

	require.Len(t, events.sent, 1)
	got := events.sent[0]
	assert.Equal(t, uid, got["user_id"])
	assert.Equal(t, rid, got["resume_id"])
	assert.Greater(t, got["count"], 0)
	assert.Equal(t, "contact_details", got["rule"], "the first rule to correct, in screen order")
}

func TestSendTipsClaimFails(t *testing.T) {
	tips := &mocks.TipRepo{ClaimFn: func(context.Context, time.Time, time.Time, int) ([]db.TipClaim, error) {
		return nil, errors.New("db down")
	}}
	_, apiErr := NewTipService(tips, &mocks.ResumeRepo{}, &mocks.TemplateRepo{}, core.LogPublisher{}).Send(context.Background())
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusInternalServerError, apiErr.Code)

	// A release that fails is logged and the run goes on.
	tips = &mocks.TipRepo{ReleaseErr: errors.New("db down"), ClaimFn: func(context.Context, time.Time, time.Time, int) ([]db.TipClaim, error) {
		return []db.TipClaim{{UserID: uid, ResumeID: rid}}, nil
	}}
	resumes := &mocks.ResumeRepo{GetByIDFn: func(context.Context, string) (*db.Resume, error) { return nil, errors.New("db down") }}
	res, apiErr := NewTipService(tips, resumes, &mocks.TemplateRepo{}, core.LogPublisher{}).Send(context.Background())
	require.Nil(t, apiErr)
	assert.Equal(t, 1, res.Failed)
}
