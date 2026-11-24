package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/ai/resume/parse", r.URL.Path)
		assert.Equal(t, "secret", r.Header.Get("x-api-key"))
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "cv text", body["text"])
		_, _ = w.Write([]byte(`{"content":{"contact":{"name":"Ines"},"skills":["Go"]},"language":"fr","missing":[]}`))
	}))
	defer srv.Close()
	p, err := New(srv.URL, "secret").Parse(context.Background(), "cv text")
	require.NoError(t, err)
	assert.Equal(t, "Ines", p.Content.Contact.Name)
	assert.Equal(t, "fr", p.Language)
}

func TestParseErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	_, err := New(srv.URL, "k").Parse(context.Background(), "x")
	assert.EqualError(t, err, "ai parse: status 503")
}
