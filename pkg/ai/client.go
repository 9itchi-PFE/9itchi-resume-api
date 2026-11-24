// Package ai is the HTTP client for 9itchi-ai-api. Base URL comes from env, never hard-coded.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"9itchi-resume-api/core"
	"9itchi-resume-api/models/api"
)

type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

func New(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTP:    &http.Client{Timeout: 90 * time.Second},
	}
}

var _ core.ResumeParser = (*Client)(nil)

// Parse calls POST /ai/resume/parse. The CV text is never logged.
func (c *Client) Parse(ctx context.Context, text string) (*core.ParsedResume, error) {
	body, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+"/ai/resume/parse", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, core.ErrPeerUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return nil, fmt.Errorf("ai parse: status %d", resp.StatusCode)
	}
	var out struct {
		Content  api.ResumeContent `json:"content"`
		Language string            `json:"language"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&out); err != nil {
		return nil, fmt.Errorf("ai parse: decode: %w", err)
	}
	return &core.ParsedResume{Content: out.Content, Language: out.Language}, nil
}
