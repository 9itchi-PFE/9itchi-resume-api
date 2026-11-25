package handlers

import (
	"net/http"

	"9itchi-resume-api/models"
	"9itchi-resume-api/models/api"
	"9itchi-resume-api/services"

	"github.com/gin-gonic/gin"
)

// ListTemplates: GET /templates (user) template gallery.
func ListTemplates(svc services.TemplateService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.ListTemplates(c.Request.Context(), c.GetString("user_id"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// ListAllTemplates: GET /admin/templates (admin) every template, published or not, with how many CVs use it.
func ListAllTemplates(svc services.TemplateService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.ListAllTemplates(c.Request.Context())
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// GetTemplate: GET /templates/:id (user) template detail.
func GetTemplate(svc services.TemplateService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.GetTemplate(c.Request.Context(), c.GetString("user_id"), c.Param("id"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// CreateTemplate: POST /admin/templates (admin) add template.
func CreateTemplate(svc services.TemplateService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.CreateTemplateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			models.FormatError(c, &models.ApiError{Code: http.StatusBadRequest, Error: err})
			return
		}
		result, apiErr := svc.CreateTemplate(c.Request.Context(), c.GetString("user_id"), req)
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusCreated, result)
	}
}

// UpdateTemplate: PUT /admin/templates/:id (admin) edit or deactivate template.
func UpdateTemplate(svc services.TemplateService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.UpdateTemplateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			models.FormatError(c, &models.ApiError{Code: http.StatusBadRequest, Error: err})
			return
		}
		result, apiErr := svc.UpdateTemplate(c.Request.Context(), c.GetString("user_id"), c.Param("id"), req)
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}
