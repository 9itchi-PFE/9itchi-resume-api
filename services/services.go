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
	// Erase removes what this service keeps about a deleted account.
	Erase    EraseService
	Resume   ResumeService
	Template TemplateService
	File     FileService
	Tip      TipService
	Rewrite  RewriteService
	Letter   LetterService
	Clients  *Clients
}

// Clients holds one HTTP client per peer service (singletons, 50s timeout).
type Clients struct {
	Ai       *ai.Client
	Account  *account.Client
	Payments *payments.Client
}

func InitServices(config utils.AppConfig, repos *repositories.Repositories) *Services {
	clients := &Clients{
		Ai:       ai.New(config.AiAPIURL, config.InternalAPIKey),
		Account:  account.New(config.AccountAPIURL, config.InternalAPIKey),
		Payments: payments.New(config.PaymentsAPIURL, config.InternalAPIKey),
	}
	// One publisher for the whole service: the weekly advice and the first CV of a student.
	events := core.NewPublisher(config.RabbitMQURL)
	return &Services{
		Erase: NewEraseService(repos.Erase),
		Resume: NewResumeService(repos.Resume, repos.Template, repos.File, repos.Store, core.DefaultExporters(),
			WithParser(clients.Ai), WithQuotas(clients.Payments), WithEvents(events)),
		Template: NewTemplateService(repos.Template),
		File:     NewFileService(repos.File, repos.Resume),
		Tip:      NewTipService(repos.Tip, repos.Resume, repos.Template, events),
		Rewrite:  NewRewriteService(repos.Rewrite, repos.Resume, clients.Ai, clients.Payments),
		Letter:   NewLetterService(repos.Resume),
		Clients:  clients,
	}
}
