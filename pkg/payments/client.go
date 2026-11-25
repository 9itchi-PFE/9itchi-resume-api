// Package payments is the HTTP client for 9itchi-payments-api. Base URL comes from env, never hard-coded.
package payments

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"9itchi-resume-api/core"
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
		HTTP:    &http.Client{Timeout: 10 * time.Second},
	}
}

var _ core.QuotaSource = (*Client)(nil)

// Quotas calls GET /internal/quotas/:userId (active plan, else Free).
func (c *Client) Quotas(ctx context.Context, userID string) (*core.Quotas, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.BaseURL, "/")+"/internal/quotas/"+url.PathEscape(userID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, core.ErrPeerUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return nil, fmt.Errorf("payments quotas: status %d", resp.StatusCode)
	}
	var out struct {
		PlanName    string         `json:"plan_name"`
		Quotas      map[string]int `json:"quotas"`
		PeriodStart *time.Time     `json:"period_start"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return nil, fmt.Errorf("payments quotas: decode: %w", err)
	}
	if out.Quotas == nil {
		out.Quotas = map[string]int{}
	}
	return &core.Quotas{PlanName: out.PlanName, Limits: out.Quotas, PeriodStart: out.PeriodStart}, nil
}
