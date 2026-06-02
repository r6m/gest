package memory

import (
	"context"
	"sync"
)

const defaultBuffer = 128

// Options configures the in-memory queue adapter.
type Options struct {
	Buffer int
}

// Adapter is an in-process queue adapter suitable for tests and local development.
type Adapter struct {
	mu     sync.Mutex
	buffer int
	queues map[string]chan any
}

// NewAdapter creates an empty in-memory adapter.
func NewAdapter(options ...Options) *Adapter {
	buffer := defaultBuffer
	if len(options) > 0 && options[0].Buffer > 0 {
		buffer = options[0].Buffer
	}
	return &Adapter{
		buffer: buffer,
		queues: make(map[string]chan any),
	}
}

// Enqueue submits a payload to a named queue.
func (a *Adapter) Enqueue(ctx context.Context, name string, payload any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	queue := a.queue(name)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case queue <- payload:
		return nil
	}
}

// Subscribe returns the delivery channel for a named queue.
func (a *Adapter) Subscribe(ctx context.Context, name string) (<-chan any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return a.queue(name), nil
}

func (a *Adapter) queue(name string) chan any {
	a.mu.Lock()
	defer a.mu.Unlock()
	queue, ok := a.queues[name]
	if !ok {
		queue = make(chan any, a.buffer)
		a.queues[name] = queue
	}
	return queue
}
