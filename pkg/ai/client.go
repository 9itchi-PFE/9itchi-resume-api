// Package ai is the HTTP client for 9itchi-ai-api. Base URL comes from env, never hard-coded.
package ai

import (
	"bytes"
	"context"
	"encoding/base64"
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
		HTTP:    &http.Client{Timeout: 200 * time.Second},
	}
}

var _ core.ResumeParser = (*Client)(nil)

// Parse calls POST /ai/resume/parse with the CV text. The CV text is never logged.
func (c *Client) Parse(ctx context.Context, text string) (*core.ParsedResume, error) {
	return c.parse(ctx, map[string]any{"text": text})
}

// ParseImage calls POST /ai/resume/parse with a photo of the CV (base64 in the JSON body).
func (c *Client) ParseImage(ctx context.Context, mediaType string, data []byte) (*core.ParsedResume, error) {
	return c.parse(ctx, map[string]any{"image": map[string]string{"media_type": mediaType, "data": base64.StdEncoding.EncodeToString(data)}})
}

func (c *Client) parse(ctx context.Context, payload map[string]any) (*core.ParsedResume, error) {
	body, err := json.Marshal(payload)
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
		Changes  struct {
			Rewritten int      `json:"rewritten"`
			Removed   []string `json:"removed"`
		} `json:"changes"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&out); err != nil {
		return nil, fmt.Errorf("ai parse: decode: %w", err)
	}
	return &core.ParsedResume{Content: out.Content, Language: out.Language, Rewritten: out.Changes.Rewritten, Removed: out.Changes.Removed}, nil
}

var _ core.Rewriter = (*Client)(nil)

// Rewrite calls POST /ai/resume/rewrite (internal) and returns its suggestions. The text is never logged.
func (c *Client) Rewrite(ctx context.Context, in core.RewriteInput) ([]string, error) {
	body, err := json.Marshal(map[string]string{"section": in.Section, "text": in.Text, "language": in.Language, "job_title": in.JobTitle})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+"/ai/resume/rewrite", bytes.NewReader(body))
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
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return nil, fmt.Errorf("%w: ai rewrite status %d", core.ErrPeerUnavailable, resp.StatusCode)
	}
	var out struct {
		Suggestions []string `json:"suggestions"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return nil, fmt.Errorf("ai rewrite: decode: %w", err)
	}
	return out.Suggestions, nil
}
