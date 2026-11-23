package api

import "time"

// CreateTemplateRequest is the body of POST /admin/templates. Active defaults to true.
type CreateTemplateRequest struct {
	Name       string `json:"name" binding:"required,max=100"`
	Layout     string `json:"layout" binding:"required,oneof=classic modern compact"`
	PreviewURL string `json:"preview_url" binding:"omitempty,max=500"`
	Active     *bool  `json:"active"`
}

// UpdateTemplateRequest is the body of PUT /admin/templates/:id. Only provided fields change.
type UpdateTemplateRequest struct {
	Name       *string `json:"name" binding:"omitempty,min=1,max=100"`
	Layout     *string `json:"layout" binding:"omitempty,oneof=classic modern compact"`
	PreviewURL *string `json:"preview_url" binding:"omitempty,max=500"`
	Active     *bool   `json:"active"`
}

type TemplateResponse struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Layout     string    `json:"layout"`
	PreviewURL string    `json:"preview_url"`
	Active     bool      `json:"active"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type ListTemplatesResponse struct {
	Items []TemplateResponse `json:"items"`
}
