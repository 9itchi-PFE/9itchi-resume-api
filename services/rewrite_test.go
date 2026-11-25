package services

import (
	"context"
	"errors"
	"net/http"
	"sync"
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

type memRewrites struct {
	mu   sync.Mutex
	rows []db.AIRewrite
	at   time.Time
}

func (m *memRewrites) Find(_ context.Context, userID, hash string) (*db.AIRewrite, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.rows {
		if m.rows[i].UserID == userID && m.rows[i].InputHash == hash {
			r := m.rows[i]
			return &r, nil
		}
	}
	return nil, repositories.ErrNotFound
}

func (m *memRewrites) Save(_ context.Context, w *db.AIRewrite) (*db.AIRewrite, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := *w
	c.CreatedAt = m.at
	m.rows = append(m.rows, c)
	return &c, nil
}

func (m *memRewrites) CountSince(_ context.Context, userID string, since time.Time) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, r := range m.rows {
		if r.UserID == userID && !r.CreatedAt.Before(since) {
			n++
		}
	}
	return n, nil
}

type fakeRewriter struct {
	calls int
	in    core.RewriteInput
	out   []string
	err   error
}

func (f *fakeRewriter) Rewrite(_ context.Context, in core.RewriteInput) ([]string, error) {
	f.calls++
	f.in = in
	return f.out, f.err
}

type limitsOf map[string]int

func (l limitsOf) Quotas(context.Context, string) (*core.Quotas, error) {
	return &core.Quotas{Limits: l}, nil
}

func newRewrite(limits map[string]int) (*rewriteService, *memRewrites, *fakeRewriter) {
	repo := &memRewrites{at: time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)}
	ai := &fakeRewriter{out: []string{"Développé en React la page de réservation."}}
	resumes := &mocks.ResumeRepo{GetFn: func(_ context.Context, userID, id string) (*db.Resume, error) {
		if userID != uid || id != rid {
			return nil, repositories.ErrNotFound
		}
		return &db.Resume{ID: rid, UserID: uid}, nil
	}}
	svc := &rewriteService{repo: repo, resumes: resumes, ai: ai, quotas: limitsOf(limits), now: func() time.Time { return time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC) }}
	return svc, repo, ai
}

func TestRewriteIsAskedOnceAndCounted(t *testing.T) {
	svc, _, ai := newRewrite(map[string]int{core.QuotaAIRewrites: 1})
	req := api.RewriteRequest{Section: "bullet", Text: " page de réservation en React "}

	res, apiErr := svc.Rewrite(context.Background(), uid, rid, req)
	require.Nil(t, apiErr)
	assert.Equal(t, []string{"Développé en React la page de réservation."}, res.Suggestions)
	require.NotNil(t, res.Limit)
	assert.Equal(t, 1, *res.Limit)
	assert.Equal(t, 1, res.Used)
	assert.Equal(t, "page de réservation en React", ai.in.Text, "trimmed")
	assert.Equal(t, "fr", ai.in.Language, "French by default")

	res, apiErr = svc.Rewrite(context.Background(), uid, rid, req)
	require.Nil(t, apiErr, "the same text comes back, even at the limit")
	assert.Equal(t, 1, ai.calls, "without asking the AI again")
	assert.Equal(t, 1, res.Used, "nor counting it")

	_, apiErr = svc.Rewrite(context.Background(), uid, rid, api.RewriteRequest{Section: "summary", Text: "autre texte"})
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusPaymentRequired, apiErr.Code)
	assert.Equal(t, errRewriteLimit, apiErr.Error.Error())
	assert.Equal(t, 1, ai.calls)
}

func TestRewriteFollowsThePlanAndTheOwner(t *testing.T) {
	req := api.RewriteRequest{Section: "bullet", Text: "t", Language: "en", JobTitle: "Stage React"}

	svc, _, ai := newRewrite(map[string]int{})
	res, apiErr := svc.Rewrite(context.Background(), uid, rid, req)
	require.Nil(t, apiErr, "no key: no limit")
	assert.Nil(t, res.Limit)
	assert.Equal(t, core.RewriteInput{Section: "bullet", Text: "t", Language: "en", JobTitle: "Stage React"}, ai.in)

	svc, _, ai = newRewrite(map[string]int{core.QuotaAIRewrites: 0})
	_, apiErr = svc.Rewrite(context.Background(), uid, rid, req)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusPaymentRequired, apiErr.Code)
	assert.Equal(t, errRewriteNeedsPlan, apiErr.Error.Error())
	assert.Zero(t, ai.calls)

	svc, _, ai = newRewrite(nil)
	_, apiErr = svc.Rewrite(context.Background(), "99999999-9999-4999-8999-999999999999", rid, req)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusNotFound, apiErr.Code, "another student's CV")
	assert.Zero(t, ai.calls)

	_, apiErr = svc.Rewrite(context.Background(), uid, "nope", req)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.Code)

	_, apiErr = svc.Rewrite(context.Background(), uid, rid, api.RewriteRequest{Section: "bullet", Text: "   "})
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.Code)

	svc, repo, ai := newRewrite(nil)
	ai.err = errors.New("ai down")
	_, apiErr = svc.Rewrite(context.Background(), uid, rid, req)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusBadGateway, apiErr.Code)
	assert.Empty(t, repo.rows, "nothing kept when the AI fails")

	svc, repo, ai = newRewrite(nil)
	ai.out = nil
	_, apiErr = svc.Rewrite(context.Background(), uid, rid, req)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusBadGateway, apiErr.Code)
	assert.Empty(t, repo.rows)
}

func TestRewritesOfLastMonthDoNotCount(t *testing.T) {
	svc, repo, _ := newRewrite(map[string]int{core.QuotaAIRewrites: 1})
	repo.at = time.Date(2026, 4, 30, 23, 0, 0, 0, time.UTC)
	_, apiErr := svc.Rewrite(context.Background(), uid, rid, api.RewriteRequest{Section: "bullet", Text: "avril"})
	require.Nil(t, apiErr)
	repo.at = time.Date(2026, 5, 2, 9, 0, 0, 0, time.UTC)
	_, apiErr = svc.Rewrite(context.Background(), uid, rid, api.RewriteRequest{Section: "bullet", Text: "mai"})
	require.Nil(t, apiErr, "April's rewrite is not counted in May")
}
