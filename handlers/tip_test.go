package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"9itchi-resume-api/models"
	"9itchi-resume-api/models/api"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

type mockTips struct{ err *models.ApiError }

func (m *mockTips) Send(context.Context) (*api.TipsResponse, *models.ApiError) {
	if m.err != nil {
		return nil, m.err
	}
	return &api.TipsResponse{Sent: 3, Failed: 1}, nil
}

func TestSendTips(t *testing.T) {
	gin.SetMode(gin.TestMode)
	call := func(svc *mockTips) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/internal/tips/send", nil)
		SendTips(svc)(c)
		return w
	}
	w := call(&mockTips{})
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"sent":3,"failed":1}`, w.Body.String())

	w = call(&mockTips{err: &models.ApiError{Code: http.StatusInternalServerError, Error: errors.New("internal error")}})
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), `"error":"internal error"`)
}
