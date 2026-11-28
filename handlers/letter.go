package handlers

import (
	"net/http"

	"9itchi-resume-api/models"
	"9itchi-resume-api/models/api"
	"9itchi-resume-api/services"

	"github.com/gin-gonic/gin"
)

// LetterPDF: POST /internal/letters/pdf (internal) the cover letter of an application as a PDF (email-api).
func LetterPDF(svc services.LetterService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.LetterRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			models.FormatError(c, &models.ApiError{Code: http.StatusBadRequest, Error: err})
			return
		}
		data, apiErr := svc.LetterPDF(c.Request.Context(), req)
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		sendNamedFile(c, &api.FileContent{Filename: "letter.pdf", ContentType: "application/pdf", Data: data})
	}
}
