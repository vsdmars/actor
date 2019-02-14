package actors

import (
	"context"
	"fmt"
	"vstmp/pkg/log"

	"go.uber.org/zap"
)

const (
	// ErrorActor actor handles errors
	ErrorActor = "errorActor"
	// DebugActor actor handles debugging
	DebugActor = "debugActor"
)

// debugActor used for debugging :-)
func debugActor(ctx context.Context, actor *Actor) {
	for {
		select {
		case <-ctx.Done():
			return
		case v := <-actor.Receive():
			fmt.Printf("print whatever we got: %v\n", v)
		}
	}
}

// errorActor processing errors
func errorActor(ctx context.Context, actor *Actor) {
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-actor.Receive():
			e := err.(error)
			log.Logger.Error(
				"error logged",
				zap.String("service", serviceName),
				zap.String("actor", ErrorActor),
				zap.String("error", e.Error()),
			)
		}
	}
}
