package actor

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

func Get(actor string) *Actor {
	return nil
}

func GetByName(actor string) *Actor {
	return Get(actor)
}

func GetByUUID(uuid string) *Actor {
	return nil
}

// NewActor creates a new actor
//
// Return new Actor if there's no error, else return nil
func NewActor(
	ctx context.Context,
	name string,
	buffer int,
	callbackFn handleType) (*Actor, error) {

	if buffer < 0 {
		return nil, ChannelBufferError
	}

	ctx, cancel := context.WithCancel(ctx)

	pipe := make(chan interface{}, buffer)

	// escape Actor object
	actor := &Actor{
		name:         name,
		uuid:         uuid.New().String(),
		actorContext: actorContext{ctx, cancel},
		channels:     channels{pipe, pipe},
	}

	/*
	   1. goroutine monitor/update idleness & persist Actor
	   2. setup host info
	   3. register actor
	*/

	go func() {
		// close Actor's channels
		defer actor.close()
		// block call
		// return closes the channel, actor dies
		callbackFn(actor)
	}()

	return actor, nil
}

// --- Actor interface functions ---

func (act *Actor) Done() <-chan struct{} {
	return act.ctx.Done()
}

func (act *Actor) SetPersistFn(fn func(interface{}) error) {
	act.persistFn = fn
}

func (act *Actor) Pure(p bool) {
	act.pure = p
}

func (act *Actor) Recycle(r bool) {
	act.recycle = r
}

func (act *Actor) IdleThreshold(t time.Duration) {
	act.idleThreshold = t
}

// Send sends message to actor
func (actor *Actor) Send(message interface{}) {
	// TODO: implement backoff
	defer func() {
		if r := recover(); r != nil {
			logger.Error(
				"actor in closed state",
				zap.String("service", serviceName),
				zap.String("actor", actor.name),
				zap.String("uuid", actor.uuid),
			)
		}
	}()

	actor.send <- message
}

// Receive receives message from actor
func (actor *Actor) Receive() <-chan interface{} {
	return actor.receive
}

// Name returns actor's name
func (actor *Actor) Name() string {
	return actor.name
}

// UUID returns actor's UUID
func (actor *Actor) UUID() string {
	return actor.uuid
}

// --- non-extern Actor member functions ---

// close closes actor's internal channel
func (actor *Actor) close() {
	close(actor.send)
}

// --- helper functions ---

// CleanUp helper function to cancel Actors
func CleanUp(actors ...*Actor) {
	for _, actor := range actors {
		actor.cancel()
	}
}
