package memory_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/r6m/gest/modules/cache/adapters/memory"
)

func TestStoreCopiesValuesOnSetAndGet(t *testing.T) {
	store := memory.NewStore()
	ctx := context.Background()
	value := []byte("value")

	if err := store.Set(ctx, "key", value, 0); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	value[0] = 'V'
	got, ok, err := store.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !ok || string(got) != "value" {
		t.Fatalf("Get = %q ok %v, want copied value hit", got, ok)
	}
	got[0] = 'V'
	again, ok, err := store.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !ok || string(again) != "value" {
		t.Fatalf("Get after mutation = %q ok %v, want stored value unchanged", again, ok)
	}
}

func TestStoreTTLExpiresValue(t *testing.T) {
	store := memory.NewStore()
	ctx := context.Background()

	if err := store.Set(ctx, "key", []byte("value"), 10*time.Millisecond); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	time.Sleep(25 * time.Millisecond)
	if _, ok, err := store.Get(ctx, "key"); err != nil || ok {
		t.Fatalf("Get expired = ok %v err %v, want miss without error", ok, err)
	}
}

func TestStoreHonorsCanceledContext(t *testing.T) {
	store := memory.NewStore()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := store.Set(ctx, "key", []byte("value"), 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("Set error = %v, want context.Canceled", err)
	}
	if _, _, err := store.Get(ctx, "key"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Get error = %v, want context.Canceled", err)
	}
	if err := store.Delete(ctx, "key"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Delete error = %v, want context.Canceled", err)
	}
}
