package handlers

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"9itchi-resume-api/models"
	"9itchi-resume-api/models/api"
	"9itchi-resume-api/services"
)

type lockSvc struct {
	services.ResumeService
	chosen string
}

func (s *lockSvc) ChooseEditable(_ context.Context, _, id string) (*api.ResumeResponse, *models.ApiError) {
	if id == "gone" {
		return nil, &models.ApiError{Code: http.StatusNotFound, Error: errors.New("resume not found")}
	}
	s.chosen = id
	return &api.ResumeResponse{ID: id}, nil
}

func TestChooseEditableHandler(t *testing.T) {
	svc := &lockSvc{}
	r := newRouter(ChooseEditable(svc), http.MethodPut, "/resumes/:id/editable")
	w := do(r, http.MethodPut, "/resumes/abc/editable", "", "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "abc", svc.chosen)
	assert.Contains(t, w.Body.String(), `"locked":false`)

	w = do(r, http.MethodPut, "/resumes/gone/editable", "", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), `"error":"resume not found"`)
}
