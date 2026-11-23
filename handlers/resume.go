package handlers

import (
	"net/http"

	"9itchi-resume-api/models"
	"9itchi-resume-api/models/api"
	"9itchi-resume-api/services"

	"github.com/gin-gonic/gin"
)

// UploadResume: POST /resumes/upload (user) upload PDF/DOCX, async AI parse.
func UploadResume(svc services.ResumeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.UploadResume(c.Request.Context(), c.GetString("user_id"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// CreateResume: POST /resumes (user) create resume from structured content.
func CreateResume(svc services.ResumeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.CreateResumeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			models.FormatError(c, &models.ApiError{Code: http.StatusBadRequest, Error: err})
			return
		}
		result, apiErr := svc.CreateResume(c.Request.Context(), c.GetString("user_id"), req)
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusCreated, result)
	}
}

// ListResumes: GET /resumes (user) list own resumes.
func ListResumes(svc services.ResumeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.ListResumes(c.Request.Context(), c.GetString("user_id"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// GetResume: GET /resumes/:id (user) resume detail.
func GetResume(svc services.ResumeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.GetResume(c.Request.Context(), c.GetString("user_id"), c.Param("id"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// UpdateResume: PUT /resumes/:id (user) edit sections / switch template.
func UpdateResume(svc services.ResumeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.UpdateResumeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			models.FormatError(c, &models.ApiError{Code: http.StatusBadRequest, Error: err})
			return
		}
		result, apiErr := svc.UpdateResume(c.Request.Context(), c.GetString("user_id"), c.Param("id"), req)
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// DeleteResume: DELETE /resumes/:id (user) delete resume.
func DeleteResume(svc services.ResumeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.DeleteResume(c.Request.Context(), c.GetString("user_id"), c.Param("id"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// ListVersions: GET /resumes/:id/versions (user) list saved versions.
func ListVersions(svc services.ResumeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.ListVersions(c.Request.Context(), c.GetString("user_id"), c.Param("id"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// CreateVersion: POST /resumes/:id/versions (user) snapshot a version.
func CreateVersion(svc services.ResumeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.CreateVersion(c.Request.Context(), c.GetString("user_id"), c.Param("id"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// ExportResume: POST /resumes/:id/export (user) export PDF or DOCX.
func ExportResume(svc services.ResumeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.ExportResumeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			models.FormatError(c, &models.ApiError{Code: http.StatusBadRequest, Error: err})
			return
		}
		result, apiErr := svc.ExportResume(c.Request.Context(), c.GetString("user_id"), c.Param("id"), req)
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// ATSCheck: POST /resumes/:id/ats-check (user) ATS score and suggestions.
func ATSCheck(svc services.ResumeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.ATSCheck(c.Request.Context(), c.GetString("user_id"), c.Param("id"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// InternalResumePDF: GET /internal/resumes/:id/pdf (internal) latest exported PDF bytes (email attachments).
func InternalResumePDF(svc services.ResumeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.InternalResumePDF(c.Request.Context(), c.Param("id"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}
