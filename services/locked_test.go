package services

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"9itchi-resume-api/core"
	"9itchi-resume-api/models/api"
	"9itchi-resume-api/models/db"
	"9itchi-resume-api/repositories"
)

const rid2 = "44444444-4444-4444-4444-444444444444"

// lockedFixture: the student has two CVs, rid first in the editable order, and a plan with the given CV limit.
func lockedFixture(q *core.Quotas, qErr error) *fixture {
	f, _ := newSprint2Fixture(WithQuotas(fakeQuotas{q: q, err: qErr}))
	f.resumes.EditableOrderFn = func(context.Context, string) ([]string, error) { return []string{rid, rid2}, nil }
	f.resumes.GetFn = func(_ context.Context, _, id string) (*db.Resume, error) {
		return &db.Resume{ID: id, UserID: uid, Title: "CV", Content: []byte(`{}`)}, nil
	}
	f.resumes.ListFn = func(context.Context, string, int, int) ([]db.ResumeSummary, int, error) {
		return []db.ResumeSummary{{ID: rid, Title: "A", UpdatedAt: time.Now()}, {ID: rid2, Title: "B"}}, 2, nil
	}
	f.resumes.UpdateFn = func(_ context.Context, _, id string, _ *string, _ *string, _ []byte) (*db.Resume, error) {
		return &db.Resume{ID: id, Content: []byte(`{}`)}, nil
	}
	return f
}

func free() *core.Quotas {
	return &core.Quotas{PlanName: core.FreePlanName, Limits: map[string]int{core.QuotaResumes: 1}}
}

func TestBeyondThePlanOnlyOneCVStaysEditable(t *testing.T) {
	f := lockedFixture(free(), nil)
	ctx := context.Background()
	title := "New"

	list, apiErr := f.svc.ListResumes(ctx, uid, 1, 20)
	require.Nil(t, apiErr)
	assert.False(t, list.Items[0].Locked)
	assert.True(t, list.Items[1].Locked)

	got, apiErr := f.svc.GetResume(ctx, uid, rid2)
	require.Nil(t, apiErr)
	assert.True(t, got.Locked, "read, not changed")

	_, apiErr = f.svc.UpdateResume(ctx, uid, rid, api.UpdateResumeRequest{Title: &title})
	assert.Nil(t, apiErr, "the editable one is changed")

	_, apiErr = f.svc.UpdateResume(ctx, uid, rid2, api.UpdateResumeRequest{Title: &title})
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusPaymentRequired, apiErr.Code)
	assert.Equal(t, "subscribe to edit your other CVs", apiErr.Error.Error())

	for name, call := range map[string]func() int{
		"version": func() int { _, e := f.svc.CreateVersion(ctx, uid, rid2, api.CreateVersionRequest{}); return e.Code },
		"restore": func() int { _, e := f.svc.RestoreVersion(ctx, uid, rid2, rid); return e.Code },
		"photo":   func() int { return f.svc.UploadPhoto(ctx, uid, rid2, strings.NewReader("\xFF\xD8\xFFjpeg")).Code },
		"no photo": func() int {
			return f.svc.DeletePhoto(ctx, uid, rid2).Code
		},
	} {
		assert.Equal(t, http.StatusPaymentRequired, call(), name)
	}
	assert.Nil(t, f.svc.DeleteResume(ctx, uid, rid2), "a locked CV can still be deleted")
}

func TestEveryCVIsEditableWithinThePlan(t *testing.T) {
	title := "New"
	for name, q := range map[string]*core.Quotas{
		"two allowed": {Limits: map[string]int{core.QuotaResumes: 2}},
		"no limit":    {Limits: map[string]int{}},
	} {
		f := lockedFixture(q, nil)
		list, apiErr := f.svc.ListResumes(context.Background(), uid, 1, 20)
		require.Nil(t, apiErr, name)
		assert.False(t, list.Items[1].Locked, name)
		_, apiErr = f.svc.UpdateResume(context.Background(), uid, rid2, api.UpdateResumeRequest{Title: &title})
		assert.Nil(t, apiErr, name)
	}
	// payments-api down: nothing is locked, the student keeps editing.
	f := lockedFixture(nil, errors.New("down"))
	_, apiErr := f.svc.UpdateResume(context.Background(), uid, rid2, api.UpdateResumeRequest{Title: &title})
	assert.Nil(t, apiErr)
	// A plan with no CV at all still leaves one editable.
	f = lockedFixture(&core.Quotas{Limits: map[string]int{core.QuotaResumes: 0}}, nil)
	_, apiErr = f.svc.UpdateResume(context.Background(), uid, rid, api.UpdateResumeRequest{Title: &title})
	assert.Nil(t, apiErr)
}

func TestChooseTheEditableCV(t *testing.T) {
	f := lockedFixture(free(), nil)
	order := []string{rid, rid2}
	f.resumes.EditableOrderFn = func(context.Context, string) ([]string, error) { return order, nil }
	f.resumes.PinFn = func(_ context.Context, userID, id string) error {
		assert.Equal(t, uid, userID)
		if id != rid && id != rid2 {
			return repositories.ErrNotFound
		}
		order = []string{id, rid}
		return nil
	}
	got, apiErr := f.svc.ChooseEditable(context.Background(), uid, rid2)
	require.Nil(t, apiErr)
	assert.False(t, got.Locked)
	got, apiErr = f.svc.GetResume(context.Background(), uid, rid)
	require.Nil(t, apiErr)
	assert.True(t, got.Locked, "the other one is now read-only")

	_, apiErr = f.svc.ChooseEditable(context.Background(), uid, "not-a-uuid")
	assert.Equal(t, http.StatusBadRequest, apiErr.Code)
	_, apiErr = f.svc.ChooseEditable(context.Background(), uid, tid)
	assert.Equal(t, http.StatusNotFound, apiErr.Code)
}

func TestWithoutTheRightToEditEveryCVIsReadOnly(t *testing.T) {
	q := &core.Quotas{PlanName: core.FreePlanName, Limits: map[string]int{core.QuotaResumes: 1, core.QuotaCVEdit: 0}}
	f := lockedFixture(q, nil)
	ctx := context.Background()
	title := "New"

	list, apiErr := f.svc.ListResumes(ctx, uid, 1, 20)
	require.Nil(t, apiErr)
	for _, it := range list.Items {
		assert.True(t, it.Locked)
		assert.Equal(t, "subscription", it.LockedReason)
	}
	got, apiErr := f.svc.GetResume(ctx, uid, rid)
	require.Nil(t, apiErr)
	assert.Equal(t, "subscription", got.LockedReason, "even the only CV")

	_, apiErr = f.svc.UpdateResume(ctx, uid, rid, api.UpdateResumeRequest{Title: &title})
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusPaymentRequired, apiErr.Code)
	assert.Equal(t, "editing your CV is part of the subscription", apiErr.Error.Error())

	for name, call := range map[string]func() *int{
		"blank CV": func() *int {
			_, e := f.svc.CreateResume(ctx, uid, api.CreateResumeRequest{Title: "CV"})
			return &e.Code
		},
		"duplicate": func() *int {
			_, e := f.svc.DuplicateResume(ctx, uid, rid, api.DuplicateResumeRequest{Title: "Copie"})
			return &e.Code
		},
		"choose": func() *int { _, e := f.svc.ChooseEditable(ctx, uid, rid2); return &e.Code },
		"photo": func() *int {
			e := f.svc.UploadPhoto(ctx, uid, rid, strings.NewReader("\xFF\xD8\xFFjpeg"))
			return &e.Code
		},
	} {
		assert.Equal(t, http.StatusPaymentRequired, *call(), name)
	}
	assert.Nil(t, f.svc.DeleteResume(ctx, uid, rid), "deleting stays possible")

	// A plan with cv_edit above 0, or without the key, edits as before.
	f = lockedFixture(&core.Quotas{Limits: map[string]int{core.QuotaResumes: 2, core.QuotaCVEdit: 1}}, nil)
	_, apiErr = f.svc.UpdateResume(ctx, uid, rid2, api.UpdateResumeRequest{Title: &title})
	assert.Nil(t, apiErr)
}
