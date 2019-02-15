package actor

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// NewActor creates new local actor
//
// ctx: caller's context
//
// name: actor's name
//
// buffer: actor's channel buffer
//
// callbackFn: actor handler
func NewActor(
	ctx context.Context,
	name string,
	buffer int,
	callbackFn HandleType) (Actor, error) {

	if buffer < 0 {
		return nil, ChannelBufferError
	}

	// create Actor's context
	ctx, cancel := context.WithCancel(ctx)
	// create Actor's message channel
	pipe := make(chan interface{}, buffer)
	logPipe := make(chan interface{}, buffer)

	// escape Actor object
	actor := Actor(
		&localActor{
			name:         name,
			uuid:         uuid.New().String(),
			actorContext: actorContext{ctx, cancel},
			channels:     channels{pipe, pipe, logPipe},
		},
	)

	if err := regActor.register(actor); err != nil {
		return nil, err
	}

	go func() {
		defer func() {
			regActor.deregister(actor)
			actor.close()
		}()

		// log receive message
		go func() {
			for {
				select {
				case <-actor.Done():
					return
				case m := <-pipe:
					logger.Debug(
						"receive",
						zap.String("service", serviceName),
						zap.String("actor", actor.Name()),
						zap.String("uuid", actor.UUID()),
						zap.Any("message", m),
					)

					logPipe <- m
				}
			}
		}()

		// block call
		// return closes the channel, actor dies
		callbackFn(actor)
	}()

	return actor, nil
}

// --- Actor interface functions ---

// Name returns actor's name
func (actor *localActor) Name() string {
	return actor.name
}

// UUID returns actor's UUID
func (actor *localActor) UUID() string {
	return actor.uuid
}

// Idle returns actor's idle time
func (actor *localActor) Idle() time.Duration {
	return actor.idle
}

// Send sends message to actor
func (actor *localActor) Send(message interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error(
				"actor in closed state",
				zap.String("service", serviceName),
				zap.String("actor", actor.name),
				zap.String("uuid", actor.uuid),
				zap.Any("recover", r),
			)

			err = ChannelClosedError
		}
	}()

	// block, force golang scheduler to process message.
	// do not use select on purpose.
	actor.send <- message

	logger.Debug(
		"send",
		zap.String("service", serviceName),
		zap.String("actor", actor.Name()),
		zap.String("uuid", actor.UUID()),
		zap.Any("message", message),
	)

	return
}

// Receive receives message from actor
func (actor *localActor) Receive() <-chan interface{} {
	return actor.logChannel
}

// Done Actor's context.done()
//
// context.done() is used for cleaning up Actor resource
func (act *localActor) Done() <-chan struct{} {
	return act.ctx.Done()
}

func (act *localActor) close() {
	close(act.logChannel)
	close(act.send)
}