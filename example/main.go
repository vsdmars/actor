package main

import (
	"context"
	"fmt"
	"time"

	"github.com/vsdmars/actor"
	"go.uber.org/zap"
)

func logActor(act actor.Actor) {
	for {
		select {
		case <-act.Done():
			fmt.Println("Sayonara, my friends~ -- logActor")
			return
		case v := <-act.Receive():
			fmt.Printf("LOG MESSAGE: %v\n", v)
		}
	}
}

func Pipe1Actor(act actor.Actor) {
	logActor, _ := actor.Get("logger")
	pipe2Actor, _ := actor.Get("pipe2")

	for {
		select {
		case <-act.Done():
			fmt.Println("Sayonara, my friends~ -- Pipe1Actor")
			return
		case msg := <-act.Receive():
			fmt.Printf("Pipe1 received message: %v\n", msg)
			go logActor.Send(fmt.Sprintf("pipe1 got message: %v", msg))
			go pipe2Actor.Send(msg)
		}
	}
}

func Pipe2Actor(act actor.Actor) {
	logActor, _ := actor.Get("logger")

	for {
		select {
		case <-act.Done():
			fmt.Println("Sayonara, my friends~ -- Pipe2Actor")
			return
		case msg := <-act.Receive():
			fmt.Printf("Pipe2 received message: %v\n", msg)
			go logActor.Send(fmt.Sprintf("pipe2 got message: %v", msg))
		}
	}
}

func test(ctx context.Context) {

	actor.NewActor(ctx, "pipe2", 3, Pipe2Actor)

	pipe1, _ := actor.NewActor(ctx, "pipe1", 3, Pipe1Actor)

	for i := 0; i < 10; i++ {
		pipe1.Send(fmt.Sprintf("message %d", i))
	}

	time.Sleep(3 * time.Second)
}

func main() {
	// actor.SetLogLevel(zap.FatalLevel)
	actor.SetLogLevel(zap.DebugLevel)

	ctx, cancel := context.WithCancel(context.Background())

	// create log actor to log actor's message
	actor.NewActor(ctx, "logger", 3, logActor)

	go func() {
		test(ctx)
		cancel()
	}()

	<-ctx.Done()
	// wait actors to clean up
	time.Sleep(3 * time.Second)
}
