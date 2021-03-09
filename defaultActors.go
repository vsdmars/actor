package actor

import (
	. "github.com/vsdmars/actor/internal/logger"

	"go.uber.org/zap"
)

// LogErrorActor used for logging errors
func LogErrorActor(actor Actor) {
	for {
		select {
		case <-actor.Done():
			return
		case err := <-actor.Receive():
			e := err.(error)
			GetLog().Error(
				"error logged",
				zap.String("service", serviceName),
				zap.String("actor", actor.Name()),
				zap.String("uuid", actor.UUID()),
				zap.String("error", e.Error()),
			)
		}
	}
}
