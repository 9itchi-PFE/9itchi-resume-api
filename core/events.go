package core

import (
	"context"

	"9itchi-resume-api/utils"

	"go.uber.org/zap"
)

// Event names exchanged over RabbitMQ (exchange "9itchi.events"). Never rename a published name.
const (
	// EventCVTips: the check of a student's CV still finds points to correct (once a week at most).
	EventCVTips = "cv.tips"
	// EventCVFirstImported: a student imported their first CV (once per student, unless they delete it and start again).
	EventCVFirstImported = "cv.first_imported"
)

// Publisher sends domain events.
type Publisher interface {
	Publish(ctx context.Context, event string, payload any) error
}

// LogPublisher logs the event name only (never the payload). Used when RABBITMQ_URL is not set.
type LogPublisher struct{}

func (LogPublisher) Publish(_ context.Context, event string, _ any) error {
	utils.Logger.Info("event", zap.String("name", event))
	return nil
}
