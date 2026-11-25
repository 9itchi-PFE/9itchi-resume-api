package services

import (
	"context"
	"net/http"
	"testing"

	"9itchi-resume-api/models/api"
	"9itchi-resume-api/models/db"
	"9itchi-resume-api/repositories"
	"9itchi-resume-api/services/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateService(t *testing.T) {
	repo := &mocks.TemplateRepo{}
	svc := NewTemplateService(repo)

	repo.ListActiveFn = func(context.Context) ([]db.Template, error) {
		return []db.Template{{ID: tid, Name: "Classic", Layout: "classic", Active: true}}, nil
	}
	list, apiErr := svc.ListTemplates(context.Background(), uid)
	require.Nil(t, apiErr)
	require.Len(t, list.Items, 1)

	_, apiErr = svc.GetTemplate(context.Background(), uid, "bad")
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.Code)

	_, apiErr = svc.GetTemplate(context.Background(), uid, tid) // mock default: not found
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusNotFound, apiErr.Code)

	var gotActive bool
	repo.CreateFn = func(_ context.Context, name, layout, _ string, active bool) (*db.Template, error) {
		gotActive = active
		return &db.Template{ID: tid, Name: name, Layout: layout, Active: active}, nil
	}
	_, apiErr = svc.CreateTemplate(context.Background(), uid, api.CreateTemplateRequest{Name: "X", Layout: "modern"})
	require.Nil(t, apiErr)
	assert.True(t, gotActive, "active defaults to true")

	repo.CreateFn = func(context.Context, string, string, string, bool) (*db.Template, error) {
		return nil, repositories.ErrConflict
	}
	_, apiErr = svc.CreateTemplate(context.Background(), uid, api.CreateTemplateRequest{Name: "X", Layout: "modern"})
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusConflict, apiErr.Code)

	off := false
	var patch repositories.TemplatePatch
	repo.UpdateFn = func(_ context.Context, id string, p repositories.TemplatePatch) (*db.Template, error) {
		patch = p
		return &db.Template{ID: id}, nil
	}
	_, apiErr = svc.UpdateTemplate(context.Background(), uid, tid, api.UpdateTemplateRequest{Active: &off})
	require.Nil(t, apiErr)
	require.NotNil(t, patch.Active)
	assert.False(t, *patch.Active)
}

func TestAdminSeesEveryTemplateWithItsUse(t *testing.T) {
	repo := &mocks.TemplateRepo{}
	svc := NewTemplateService(repo)
	repo.ListAllFn = func(context.Context) ([]db.TemplateUsage, error) {
		return []db.TemplateUsage{
			{Template: db.Template{ID: tid, Name: "Classic", Layout: "classic", Active: true}, Resumes: 12},
			{Template: db.Template{ID: "t2", Name: "Compact", Layout: "compact"}},
		}, nil
	}
	list, apiErr := svc.ListAllTemplates(context.Background())
	require.Nil(t, apiErr)
	require.Len(t, list.Items, 2)
	assert.Equal(t, 12, list.Items[0].Resumes)
	assert.True(t, list.Items[0].Active)
	assert.False(t, list.Items[1].Active, "a withdrawn template is listed too")

	repo.ListAllFn = func(context.Context) ([]db.TemplateUsage, error) { return nil, assert.AnError }
	_, apiErr = svc.ListAllTemplates(context.Background())
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
}

func TestLastPublishedTemplateStays(t *testing.T) {
	repo := &mocks.TemplateRepo{}
	svc := NewTemplateService(repo)
	updated := 0
	repo.UpdateFn = func(_ context.Context, id string, _ repositories.TemplatePatch) (*db.Template, error) {
		updated++
		return &db.Template{ID: id}, nil
	}
	off, on, none := false, true, 0
	repo.OtherActive = &none

	_, apiErr := svc.UpdateTemplate(context.Background(), uid, tid, api.UpdateTemplateRequest{Active: &off})
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusConflict, apiErr.Code)
	assert.Zero(t, updated, "nothing is written")

	_, apiErr = svc.UpdateTemplate(context.Background(), uid, tid, api.UpdateTemplateRequest{Active: &on})
	require.Nil(t, apiErr, "publishing is never blocked")
	name := "Classique"
	_, apiErr = svc.UpdateTemplate(context.Background(), uid, tid, api.UpdateTemplateRequest{Name: &name})
	require.Nil(t, apiErr, "renaming is not withdrawing")
	assert.Equal(t, 2, updated)
}
