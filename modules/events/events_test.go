package events_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/r6m/gest/modules/events"
)

func TestBusEmitCallsListenersSynchronouslyInOrder(t *testing.T) {
	bus := events.NewBus()
	calls := []string{}
	if err := bus.On("user.created", func(ctx context.Context, payload any) error {
		value, ok := payload.(string)
		if !ok {
			t.Fatalf("payload = %T, want string", payload)
		}
		calls = append(calls, value+"-first")
		return nil
	}); err != nil {
		t.Fatalf("On returned error: %v", err)
	}
	if err := bus.On("user.created", func(ctx context.Context, payload any) error {
		value, ok := payload.(string)
		if !ok {
			t.Fatalf("payload = %T, want string", payload)
		}
		calls = append(calls, value+"-second")
		return nil
	}); err != nil {
		t.Fatalf("On returned error: %v", err)
	}

	if err := bus.Emit(context.Background(), "user.created", "event"); err != nil {
		t.Fatalf("Emit returned error: %v", err)
	}

	want := []string{"event-first", "event-second"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func TestBusEmitStopsOnListenerError(t *testing.T) {
	bus := events.NewBus()
	failure := errors.New("failed")
	if err := bus.On("user.created", func(ctx context.Context, payload any) error {
		return failure
	}); err != nil {
		t.Fatalf("On returned error: %v", err)
	}

	err := bus.Emit(context.Background(), "user.created", "event")
	if !errors.Is(err, failure) {
		t.Fatalf("Emit error = %v, want wrapped listener error", err)
	}
}

func TestHandleRejectsWrongPayloadType(t *testing.T) {
	listener := events.Handle(func(ctx context.Context, event testEvent) error {
		return nil
	})

	err := listener(context.Background(), "wrong")
	if err == nil {
		t.Fatal("listener returned nil error for wrong payload")
	}
}

type testEvent struct{}
