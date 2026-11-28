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
	"9itchi-resume-api/repositories"
)

type usageQuotas struct{ q core.Quotas }

func (u usageQuotas) Quotas(context.Context, string) (*core.Quotas, error) { return &u.q, nil }

type usageResumes struct {
	repositories.ResumeRepository
	got string
}

func (r *usageResumes) CountByUser(_ context.Context, id string) (int, error) {
	r.got = id
	return 2, nil
}

type usageFiles struct {
	repositories.FileRepository
	since *time.Time
}

func (f *usageFiles) CountExportsByUser(_ context.Context, _ string, since *time.Time) (int, error) {
	f.since = since
	return 3, nil
}

type usageRewrites struct {
	repositories.RewriteRepository
	since time.Time
	err   error
}

func (r *usageRewrites) CountSince(_ context.Context, _ string, since time.Time) (int, error) {
	r.since = since
	return 9, r.err
}

func TestUserUsageForTheTeam(t *testing.T) {
	const student = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	period := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	resumes, files, rewrites := &usageResumes{}, &usageFiles{}, &usageRewrites{}
	svc := NewUsageService(UsageDeps{
		Quotas:  usageQuotas{core.Quotas{PeriodStart: &period, Limits: map[string]int{core.QuotaResumes: 5, core.QuotaCVDownloads: 0, core.QuotaAIRewrites: -1}}},
		Resumes: resumes, Files: files, Rewrites: rewrites,
		Now: func() time.Time { return time.Date(2026, 11, 28, 10, 0, 0, 0, time.UTC) },
	})

	res, apiErr := svc.UserUsage(context.Background(), strings.ToUpper(student))
	require.Nil(t, apiErr)
	five, none := 5, 0
	assert.Equal(t, &api.UserUsageResponse{
		Resumes:    api.UsageItem{Limit: &five, Used: 2},
		Downloads:  api.UsageItem{Limit: &none, Used: 3},
		AIRewrites: api.UsageItem{Used: 9},
	}, res, "0 is a limit (none allowed), a negative one is no limit")
	assert.Equal(t, student, resumes.got)
	assert.Equal(t, &period, files.since, "downloads count from the start of the subscription period")
	assert.Equal(t, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), rewrites.since)

	_, apiErr = svc.UserUsage(context.Background(), "nope")
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.Code)

	rewrites.err = errors.New("db down")
	_, apiErr = svc.UserUsage(context.Background(), student)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
}
