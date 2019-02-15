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

	// escape Actor object
	actor := Actor(
		&localActor{
			name:         name,
			uuid:         uuid.New().String(),
			actorContext: actorContext{ctx, cancel},
			channels:     channels{pipe, pipe},
		},
	)

	if err := regActor.register(actor); err != nil {
		return nil, err
	}

	go func() {
		defer func() {
			regActor.deregister(actor)
			actor.close()
			actor.endStamp()
		}()

		actor.startStamp()

		// increase idle by 1 second
		go func() {
			for {
				select {
				case <-actor.Done():
					return
				default:
					actor.increaseIdle()
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

	actor.resetIdle()

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
	return actor.receive
}

// Done Actor's context.done()
//
// context.done() is used for cleaning up Actor resource
func (act *localActor) Done() <-chan struct{} {
	return act.ctx.Done()
}

func (act *localActor) close() {
	act.cancel()
	close(act.send)
}

func (act *localActor) resetIdle() {
	act.idle = 0
}

func (act *localActor) increaseIdle() {
	time.Sleep(1 * time.Second)
	act.idle += 1 * time.Second

	logger.Debug(
		"actor idle seconds",
		zap.String("service", serviceName),
		zap.String("actor", act.name),
		zap.String("uuid", act.uuid),
		zap.Float64("seconds", act.idle.Seconds()),
	)
}

func (act *localActor) startStamp() {
	act.startTime = time.Now()
	logger.Debug(
		"actor start time",
		zap.String("service", serviceName),
		zap.String("actor", act.name),
		zap.String("uuid", act.uuid),
		zap.String("time", act.startTime.Format(time.UnixDate)),
	)
}

func (act *localActor) endStamp() {
	act.endTime = time.Now()
	logger.Debug(
		"actor end time",
		zap.String("service", serviceName),
		zap.String("actor", act.name),
		zap.String("uuid", act.uuid),
		zap.String("time", act.endTime.Format(time.UnixDate)),
	)
}
