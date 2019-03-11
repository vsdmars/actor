package actor

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// NewActor creates new local actor
//
// ctx: caller's context, able to cancel created actor
//
// name: actor's name
//
// buffer: actor's channel buffer
//
// callbackFn: actor handler
func NewActor(
	ctx context.Context, // caller's context, able to cancel created actor.
	name string, // actor's name
	buffer int, // actor's channel buffer
	callbackFn HandleType, // actor's handler
) (Actor, error) {

	if buffer < 0 {
		return nil, ErrChannelBuffer
	}

	// create Actor's context
	ctx, cancel := context.WithCancel(ctx)
	// create Actor's message channel
	pipe := make(chan interface{}, buffer)

	// escape Actor object, store ptr to localActor instance into Actor interface
	actor := Actor(
		&localActor{
			name:         name,
			uuid:         uuid.New().String(),
			actorContext: actorContext{ctx, cancel},
			channels:     channels{pipe, pipe},
		},
	)

	if err := regActor.register(actor); err != nil {
		actor.close() // clean up actor

		logger.Debug(
			"clean up tmp actor",
			zap.String("service", serviceName),
			zap.String("actor", actor.Name()),
			zap.String("uuid", actor.UUID()),
			zap.String("error", err.Error()),
		)

		return nil, err
	}

	go func() {
		defer func() {
			regActor.deregister(actor)
			actor.close()
			actor.endStamp()
		}()

		actor.startStamp()

		go actor.increaseIdle()

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
	return time.Duration(atomic.LoadInt64(&actor.idle))
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

			err = ErrChannelClosed
		}
	}()

	select {
	case <-actor.Done():
		logger.Error(
			"actor is cancelled",
			zap.String("service", serviceName),
			zap.String("actor", actor.name),
			zap.String("uuid", actor.uuid),
			zap.String("error", "actor is cancelled"),
		)

		return
	default:
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
}

// Receive receives message from actor
func (actor *localActor) Receive() <-chan interface{} {
	return actor.receive
}

// Done Actor's context.done()
//
// context.done() is used for cleaning up Actor resource
func (actor *localActor) Done() <-chan struct{} {
	return actor.ctx.Done()
}

func (actor *localActor) close() {
	actor.cancel()
	// https://stackoverflow.com/a/8593986
	// do not close actor's channel avoid race condition
	// it's not a resource leak if channel remains open
	// close(act.send)
}

func (actor *localActor) resetIdle() {
	atomic.StoreInt64(&actor.idle, 0)
}

func (actor *localActor) increaseIdle() {
	if actor.timer == nil {
		actor.timer = time.NewTimer(10 * time.Second)
	}

	for {
		select {
		case <-actor.Done():
			// clean up the timer
			if actor.timer != nil {
				actor.timer.Stop()
			}

			return
		case passed := <-actor.timer.C:
			atomic.AddInt64(
				&actor.idle,
				int64(time.Duration(passed.Second())*time.Second),
			)

			logger.Debug(
				"actor idle seconds",
				zap.String("service", serviceName),
				zap.String("actor", actor.name),
				zap.String("uuid", actor.uuid),
				zap.Float64("seconds", time.Duration(
					atomic.LoadInt64(&actor.idle)).Seconds()),
			)

			actor.timer.Reset(10 * time.Second)
		}
	}
}

func (actor *localActor) startStamp() {
	actor.startTime = time.Now()

	logger.Info(
		"actor start time",
		zap.String("service", serviceName),
		zap.String("actor", actor.name),
		zap.String("uuid", actor.uuid),
		zap.String("time", actor.startTime.Format(time.UnixDate)),
	)
}

func (actor *localActor) endStamp() {
	actor.endTime = time.Now()

	logger.Info(
		"actor end time",
		zap.String("service", serviceName),
		zap.String("actor", actor.name),
		zap.String("uuid", actor.uuid),
		zap.String("time", actor.endTime.Format(time.UnixDate)),
	)

}
