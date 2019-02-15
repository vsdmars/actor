package actor

import "errors"

var (
	// ChannelBufferError channel buffer setting error
	ChannelBufferError = errors.New("channel buffer error")
	// ChannelClosedError channel is in closed state
	ChannelClosedError = errors.New("channel in closed state error")
	// RegisterActorError register actor error
	RegisterActorError = errors.New("register actor error")
	// RetrieveActorError retrieve actor error
	RetrieveActorError = errors.New("retrieve actor error")
	// SendError actor send message error
	SendError = errors.New("send message error")
)
