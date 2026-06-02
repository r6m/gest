package queue

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/r6m/gest"
	"github.com/r6m/gest/modules/queue/adapters/memory"
)

// Options configures the optional queue module.
type Options struct {
	Adapter Adapter
}

// Adapter is the queue storage/delivery contract used by Queue.
type Adapter interface {
	Enqueue(ctx context.Context, name string, payload any) error
	Subscribe(ctx context.Context, name string) (<-chan any, error)
}

// Module returns a Gest module that provides an in-process queue through DI.
func Module(options Options) gest.Module {
	return gest.NewModule(gest.ModuleConfig{
		Name: "QueueModule",
		Providers: gest.Providers(
			gest.Provide(func() *Queue {
				return NewQueue(options)
			}),
		),
	})
}

// Queue delivers jobs to registered processors.
type Queue struct {
	mu         sync.Mutex
	adapter    Adapter
	processors []ProcessorBinding
	started    bool
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// NewQueue creates a queue using the configured adapter or memory by default.
func NewQueue(options Options) *Queue {
	adapter := options.Adapter
	if adapter == nil {
		adapter = memory.NewAdapter()
	}
	return &Queue{adapter: adapter}
}

// Handler processes one queued payload.
type Handler func(context.Context, any) error

// ProcessorBinding describes one generated queue processor binding.
type ProcessorBinding struct {
	Queue  string
	Handle Handler
}

// ProcessorDefinition describes generated processor metadata.
type ProcessorDefinition struct {
	Name       string
	Processors []ProcessorBinding
}

// DescribedProcessor is implemented by generated processor metadata.
type DescribedProcessor interface {
	GestQueueProcessor() ProcessorDefinition
}

// RegisterProcessor registers generated metadata from a processor provider.
func RegisterProcessor(queue *Queue, processor DescribedProcessor) error {
	if queue == nil {
		return fmt.Errorf("QUEUE_INVALID_QUEUE: queue is nil")
	}
	if processor == nil {
		return fmt.Errorf("QUEUE_INVALID_PROCESSOR: processor is nil")
	}
	definition := processor.GestQueueProcessor()
	if definition.Name == "" {
		return fmt.Errorf("QUEUE_INVALID_PROCESSOR: processor name is empty")
	}
	for _, binding := range definition.Processors {
		if err := queue.AddProcessor(binding); err != nil {
			return err
		}
	}
	return nil
}

// AddProcessor registers a processor. It must be called before queue startup.
func (q *Queue) AddProcessor(binding ProcessorBinding) error {
	if q == nil {
		return fmt.Errorf("QUEUE_INVALID_QUEUE: queue is nil")
	}
	if binding.Queue == "" {
		return fmt.Errorf("QUEUE_INVALID_PROCESSOR: queue name is empty")
	}
	if binding.Handle == nil {
		return fmt.Errorf("QUEUE_INVALID_PROCESSOR: processor for %q is nil", binding.Queue)
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.started {
		return fmt.Errorf("QUEUE_ALREADY_STARTED: processor for %q cannot be registered after startup", binding.Queue)
	}
	q.processors = append(q.processors, binding)
	return nil
}

// Enqueue submits a payload to a named queue.
func (q *Queue) Enqueue(ctx context.Context, name string, payload any) error {
	if q == nil || q.adapter == nil {
		return fmt.Errorf("QUEUE_INVALID_QUEUE: queue adapter is nil")
	}
	if name == "" {
		return fmt.Errorf("QUEUE_INVALID_NAME: queue name is empty")
	}
	return q.adapter.Enqueue(ctx, name, payload)
}

// OnModuleInit starts queue workers after processor providers register metadata.
func (q *Queue) OnModuleInit(ctx context.Context) error {
	_ = ctx
	return q.Start()
}

// BeforeApplicationShutdown stops queue workers.
func (q *Queue) BeforeApplicationShutdown(ctx context.Context) error {
	return q.Shutdown(ctx)
}

// Start begins processor delivery loops.
func (q *Queue) Start() error {
	if q == nil || q.adapter == nil {
		return fmt.Errorf("QUEUE_INVALID_QUEUE: queue adapter is nil")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.started {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	q.cancel = cancel
	q.started = true
	for _, binding := range q.processors {
		jobs, err := q.adapter.Subscribe(ctx, binding.Queue)
		if err != nil {
			cancel()
			q.started = false
			return err
		}
		q.wg.Add(1)
		go runProcessor(ctx, jobs, binding.Handle, &q.wg)
	}
	return nil
}

// Shutdown stops processor delivery loops.
func (q *Queue) Shutdown(ctx context.Context) error {
	if q == nil {
		return nil
	}
	q.mu.Lock()
	if !q.started {
		q.mu.Unlock()
		return nil
	}
	cancel := q.cancel
	q.cancel = nil
	q.started = false
	q.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	done := make(chan struct{})
	go func() {
		q.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func runProcessor(ctx context.Context, jobs <-chan any, handler Handler, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case payload, ok := <-jobs:
			if !ok {
				return
			}
			_ = handler(ctx, payload)
		}
	}
}

// Handle adapts a typed Process method to queue processor metadata.
func Handle[T any](handler func(context.Context, T) error) Handler {
	return func(ctx context.Context, payload any) error {
		job, ok := payload.(T)
		if !ok {
			return fmt.Errorf("QUEUE_INVALID_PAYLOAD: got %s, want %s", typeName(payload), typeNameOf[T]())
		}
		return handler(ctx, job)
	}
}

func typeName(value any) string {
	if value == nil {
		return "<nil>"
	}
	return reflect.TypeOf(value).String()
}

func typeNameOf[T any]() string {
	var zero *T
	return reflect.TypeOf(zero).Elem().String()
}
