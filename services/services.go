package services

import (
	"9itchi-resume-api/core"
	"9itchi-resume-api/pkg/account"
	"9itchi-resume-api/pkg/ai"
	"9itchi-resume-api/pkg/payments"
	"9itchi-resume-api/repositories"
	"9itchi-resume-api/utils"
)

// Services holds service implementations and the peer clients they call.
type Services struct {
	Resume   ResumeService
	Template TemplateService
	File     FileService
	Clients  *Clients
}

// Clients holds one HTTP client per peer service (singletons, 50s timeout).
type Clients struct {
	Ai       *ai.Client
	Account  *account.Client
	Payments *payments.Client
}

func InitServices(config utils.AppConfig, repos *repositories.Repositories) *Services {
	return &Services{
		Resume:   NewResumeService(repos.Resume, repos.Template, repos.File, repos.Store, core.DefaultExporters()),
		Template: NewTemplateService(repos.Template),
		File:     NewFileService(repos.File, repos.Resume),
		Clients: &Clients{
			Ai:       ai.New(config.AiAPIURL, config.InternalAPIKey),
			Account:  account.New(config.AccountAPIURL, config.InternalAPIKey),
			Payments: payments.New(config.PaymentsAPIURL, config.InternalAPIKey),
		},
	}
}
