package payments

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQuotas(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/internal/quotas/u-1", r.URL.Path)
		assert.Equal(t, "secret", r.Header.Get("x-api-key"))
		_, _ = w.Write([]byte(`{"user_id":"u-1","plan_id":"p","plan_name":"Free","quotas":{"resumes":1,"cv_downloads":1}}`))
	}))
	defer srv.Close()
	q, err := New(srv.URL, "secret").Quotas(context.Background(), "u-1")
	require.NoError(t, err)
	assert.Equal(t, "Free", q.PlanName)
	assert.Equal(t, map[string]int{"resumes": 1, "cv_downloads": 1}, q.Limits)
}

func TestQuotasErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	_, err := New(srv.URL, "k").Quotas(context.Background(), "u")
	assert.EqualError(t, err, "payments quotas: status 500")

	_, err = New("http://127.0.0.1:1", "k").Quotas(context.Background(), "u")
	assert.Error(t, err)
}
