package actor

import (
	"context"
	"net"
	"sync"
	"time"
)

type (
	actorContext struct {
		ctx    context.Context
		cancel context.CancelFunc
	}

	channels struct {
		send    chan<- interface{}
		receive <-chan interface{}
	}

	host struct {
		dn    net.NS // domain name
		ipv4  net.IP // ipv4
		ipv6  net.IP // ipv6
		proto string // type not settled yet, e.g: gRPC / websocket / https / http / QUIC
	}

	// event sourcing, play back queue log from persisted logs
	persist struct {
		persistFn func(data interface{}) error // use json.Marshal to store data
		pure      bool                         // pure function/actor or not. (stateful/stateless)
	}

	timing struct {
		startTime time.Time
		endTime   time.Time
		idle      time.Duration
	}

	recycle struct {
		idleThreshold time.Duration // max idle time to recycle
		recycle       bool          // should actor die after idle for a period
	}
)

type (
	// Actor provides several member functions to interact with Actor
	Actor struct {
		name string
		uuid string
		actorContext
		host
		persist
		recycle
		timing
		channels
	}
)

type (
	registeredActor struct {
		mLock     sync.Mutex
		nameUUID  map[string]string
		uuidActor map[string]*Actor
	}

	handleType func(*Actor)
)
