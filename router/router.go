package router

import (
	"9itchi-resume-api/handlers"
	"9itchi-resume-api/services"

	"github.com/gin-gonic/gin"
)

func InitializeRouter(services *services.Services) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	initializeMiddlewares(r)
	initializeRoutes(r, services)

	return r
}

func initializeRoutes(r *gin.Engine, services *services.Services) {
	// Health endpoints
	r.GET("/", handlers.GetHealth)
	r.GET("/health", handlers.GetHealth)

	// user endpoints
	userGroup := r.Group("/")
	userGroup.Use(AuthMiddleware())
	{
		userGroup.POST("/resumes/upload", handlers.UploadResume(services.Resume))
		userGroup.POST("/resumes", handlers.CreateResume(services.Resume))
		userGroup.GET("/resumes", handlers.ListResumes(services.Resume))
		userGroup.GET("/resumes/:id", handlers.GetResume(services.Resume))
		userGroup.PUT("/resumes/:id", handlers.UpdateResume(services.Resume))
		userGroup.DELETE("/resumes/:id", handlers.DeleteResume(services.Resume))
		userGroup.GET("/resumes/:id/versions", handlers.ListVersions(services.Resume))
		userGroup.POST("/resumes/:id/versions", handlers.CreateVersion(services.Resume))
		userGroup.POST("/resumes/:id/rewrite", handlers.Rewrite(services.Rewrite))
		userGroup.GET("/resumes/:id/versions/:versionId", handlers.GetVersion(services.Resume))
		userGroup.POST("/resumes/:id/versions/:versionId/restore", handlers.RestoreVersion(services.Resume))
		userGroup.POST("/resumes/:id/duplicate", handlers.DuplicateResume(services.Resume))
		userGroup.PUT("/resumes/:id/editable", handlers.ChooseEditable(services.Resume))
		userGroup.PUT("/resumes/:id/photo", handlers.UploadPhoto(services.Resume))
		userGroup.GET("/resumes/:id/photo", handlers.GetPhoto(services.Resume))
		userGroup.DELETE("/resumes/:id/photo", handlers.DeletePhoto(services.Resume))
		userGroup.POST("/resumes/:id/export", handlers.ExportResume(services.Resume))
		userGroup.POST("/resumes/:id/ats-check", handlers.ATSCheck(services.Resume))
		userGroup.GET("/me/downloads", handlers.Downloads(services.Resume))
		userGroup.GET("/templates", handlers.ListTemplates(services.Template))
		userGroup.GET("/templates/:id", handlers.GetTemplate(services.Template))
		userGroup.GET("/resumes/:id/files", handlers.ListFiles(services.File))
		userGroup.GET("/resumes/:id/files/:fileId", handlers.DownloadFile(services.File))
		userGroup.DELETE("/resumes/:id/files/:fileId", handlers.DeleteFile(services.File))
	}

	// admin endpoints
	adminGroup := r.Group("/")
	adminGroup.Use(AuthMiddleware(), RequireRole("admin"))
	{
		adminGroup.GET("/admin/templates", handlers.ListAllTemplates(services.Template))
		adminGroup.POST("/admin/templates", handlers.CreateTemplate(services.Template))
		adminGroup.PUT("/admin/templates/:id", handlers.UpdateTemplate(services.Template))
		adminGroup.GET("/admin/users/:id/cv", handlers.UserCV(services.Resume))
		adminGroup.GET("/admin/users/:id/downloads", handlers.UserDownloads(services.Resume))
		adminGroup.GET("/admin/users/:id/usage", handlers.UserUsage(services.Usage))
	}

	// internal endpoints
	internalGroup := r.Group("/")
	internalGroup.Use(InternalAPIKeyMiddleware())
	{
		internalGroup.DELETE("/internal/users/:id", handlers.EraseUser(services.Erase))
		internalGroup.GET("/internal/resumes/:id/pdf", handlers.InternalResumePDF(services.Resume))
		internalGroup.GET("/internal/resumes/:id/original", handlers.InternalResumeOriginal(services.Resume))
		internalGroup.POST("/internal/tips/send", handlers.SendTips(services.Tip))
		internalGroup.POST("/internal/letters/pdf", handlers.LetterPDF(services.Letter))
	}

	// Fallback
	r.NoRoute(handlers.NoRoute)
}
