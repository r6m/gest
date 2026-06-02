package memory_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/r6m/gest/modules/queue/adapters/memory"
)

func TestAdapterEnqueueDeliversToSubscriber(t *testing.T) {
	adapter := memory.NewAdapter()
	ctx := context.Background()
	jobs, err := adapter.Subscribe(ctx, "email")
	if err != nil {
		t.Fatalf("Subscribe returned error: %v", err)
	}
	if err := adapter.Enqueue(ctx, "email", "payload"); err != nil {
		t.Fatalf("Enqueue returned error: %v", err)
	}

	select {
	case got := <-jobs:
		if got != "payload" {
			t.Fatalf("job = %#v, want payload", got)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("subscriber did not receive job")
	}
}

func TestAdapterHonorsCanceledContext(t *testing.T) {
	adapter := memory.NewAdapter()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := adapter.Enqueue(ctx, "email", "payload"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Enqueue error = %v, want context.Canceled", err)
	}
	if _, err := adapter.Subscribe(ctx, "email"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Subscribe error = %v, want context.Canceled", err)
	}
}
