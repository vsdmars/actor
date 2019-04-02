package actor

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/google/uuid"
)

type testCase struct {
	name   string // actor name
	buffer int    // actor buffer
	backup int    // actor backup
}

const (
	createActorError = "create actor error"
	getActorError    = "get actor error"
)

func createHandle(t *testing.T, expect string, cnt int) (func(Actor), <-chan struct{}) {
	done := make(chan struct{})

	handle := func(act Actor) {
		count := 0

		for {
			select {
			case <-act.Done():
				return
			case d := <-act.Receive():
				s, ok := d.(string)
				if !ok {
					t.Error("receiving message with wrong type")
				}

				if s != expect {
					t.Errorf("expecting: %s, receiving: %s", expect, s)
				}

				count += 1
				if count == cnt {
					done <- struct{}{}
				}
			}
		}
	}

	return handle, done
}

func createTestCase(number int, buffer, backup int) []testCase {
	tc := make([]testCase, number)
	name := "actor_%d_%s"

	for idx := 0; idx < number; idx++ {
		tc[idx] = testCase{
			fmt.Sprintf(name, idx, uuid.New().String()),
			buffer,
			backup,
		}
	}

	return tc
}

func TestSingleActor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msg := "i am the lead role!"

	for _, tc := range createTestCase(1, 3, -1) {
		handle, done := createHandle(t, msg, 3)

		act, err := NewActor(
			ctx,
			tc.name,
			tc.buffer,
			handle,
			tc.backup,
		)
		if err != nil {
			t.Error(createActorError)
		}

		act.Send(msg)
		act.Send(msg)
		act.Send(msg)

		<-done
	}
}

func TestSingleActorBurst(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msg := "i am the lead role!"
	burst := 42000

	for _, tc := range createTestCase(1, 3, -1) {
		handle, done := createHandle(t, msg, burst)

		act, err := NewActor(
			ctx,
			tc.name,
			tc.buffer,
			handle,
			tc.backup,
		)
		if err != nil {
			t.Error(createActorError)
		}

		for idx := 0; idx < burst; idx++ {
			act.Send(msg)
		}

		<-done
	}
}

func TestSingleActorMultiSend(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msg := "i am the lead role!"
	burst := 42000

	sender := func(name string, number int) {
		act, err := Get(name)
		if err != nil {
			t.Error(getActorError)
			return
		}

		for idx := 0; idx < number; idx++ {
			act.Send(msg)
		}
	}

	for _, tc := range createTestCase(1, 3, -1) {
		handle, done := createHandle(t, msg, burst)

		act, err := NewActor(
			ctx,
			tc.name,
			tc.buffer,
			handle,
			tc.backup,
		)
		if err != nil {
			t.Error(createActorError)
		}

		dispatch := burst
		for dispatch > 0 {
			n := int(rand.Uint32()) % dispatch
			if n == 0 {
				n = dispatch
			}

			go sender(act.Name(), n)

			dispatch -= n
		}

		<-done
	}
}

func TestMultiActorMultiSend(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msg := "i am the lead role!"
	burst := 42000

	sender := func(name string, number int) {
		act, err := Get(name)
		if err != nil {
			t.Error(getActorError)
			return
		}

		for idx := 0; idx < number; idx++ {
			act.Send(msg)
		}
	}

	t.Run("MultiActorMultiSend",
		func(t *testing.T) {
			for idx, tc := range createTestCase(100, 3, -1) {
				tc := tc

				h := func(t *testing.T) {
					t.Parallel()

					handle, done := createHandle(t, msg, burst)

					act, err := NewActor(
						ctx,
						tc.name,
						tc.buffer,
						handle,
						tc.backup,
					)
					if err != nil {
						t.Error(createActorError)
					}

					dispatch := burst
					for dispatch > 0 {
						n := int(rand.Uint32()) % dispatch
						if n == 0 {
							n = dispatch
						}

						go sender(act.Name(), n)

						dispatch -= n
					}

					<-done
				}

				t.Run(fmt.Sprintf("MultiActorMultiSend_%d", idx), h)
			}
		},
	)
}
