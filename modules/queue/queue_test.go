package queue_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/r6m/gest/modules/queue"
)

func TestQueueProcessesEnqueuedPayload(t *testing.T) {
	q := queue.NewQueue(queue.Options{})
	calls := make(chan testPayload, 1)
	if err := q.AddProcessor(queue.ProcessorBinding{
		Queue: "email",
		Handle: queue.Handle(func(ctx context.Context, payload testPayload) error {
			calls <- payload
			return nil
		}),
	}); err != nil {
		t.Fatalf("AddProcessor returned error: %v", err)
	}
	if err := q.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	defer shutdownQueue(t, q)

	if err := q.Enqueue(context.Background(), "email", testPayload{ID: "job-1"}); err != nil {
		t.Fatalf("Enqueue returned error: %v", err)
	}
	select {
	case got := <-calls:
		if got.ID != "job-1" {
			t.Fatalf("payload ID = %q, want job-1", got.ID)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("processor did not receive job")
	}
}

func TestRegisterProcessorUsesGeneratedMetadata(t *testing.T) {
	q := queue.NewQueue(queue.Options{})
	processor := &describedProcessor{calls: make(chan testPayload, 1)}
	if err := queue.RegisterProcessor(q, processor); err != nil {
		t.Fatalf("RegisterProcessor returned error: %v", err)
	}
	if err := q.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	defer shutdownQueue(t, q)

	if err := q.Enqueue(context.Background(), "email", testPayload{ID: "job-1"}); err != nil {
		t.Fatalf("Enqueue returned error: %v", err)
	}
	select {
	case got := <-processor.calls:
		if got.ID != "job-1" {
			t.Fatalf("payload ID = %q, want job-1", got.ID)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("processor did not receive job")
	}
}

func TestQueueHandleRejectsWrongPayloadType(t *testing.T) {
	handler := queue.Handle(func(ctx context.Context, payload testPayload) error {
		return nil
	})
	err := handler(context.Background(), "wrong")
	if err == nil {
		t.Fatal("handler returned nil error for wrong payload")
	}
}

func TestQueueShutdownHonorsContext(t *testing.T) {
	q := queue.NewQueue(queue.Options{})
	if err := q.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := q.Shutdown(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("Shutdown error = %v, want nil or context.Canceled", err)
	}
}

func TestQueueShutdownStopsProcessorWorkers(t *testing.T) {
	q := queue.NewQueue(queue.Options{})
	calls := make(chan testPayload, 2)
	if err := q.AddProcessor(queue.ProcessorBinding{
		Queue: "email",
		Handle: queue.Handle(func(ctx context.Context, payload testPayload) error {
			calls <- payload
			return nil
		}),
	}); err != nil {
		t.Fatalf("AddProcessor returned error: %v", err)
	}
	if err := q.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if err := q.Enqueue(context.Background(), "email", testPayload{ID: "before"}); err != nil {
		t.Fatalf("Enqueue returned error: %v", err)
	}
	select {
	case <-calls:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("processor did not run before shutdown")
	}
	if err := q.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}
	if err := q.Enqueue(context.Background(), "email", testPayload{ID: "after"}); err != nil {
		t.Fatalf("Enqueue after shutdown returned error: %v", err)
	}
	select {
	case payload := <-calls:
		t.Fatalf("processor ran after shutdown with payload %#v", payload)
	case <-time.After(40 * time.Millisecond):
	}
}

func TestCoreRuntimeDoesNotImportQueueModule(t *testing.T) {
	root := projectRoot(t)
	files := []string{
		"app.go",
		"container.go",
		"module.go",
		"provider.go",
		"controller.go",
	}
	for _, file := range files {
		content := readFile(t, filepath.Join(root, file))
		if strings.Contains(content, "github.com/r6m/gest/modules/queue") {
			t.Fatalf("core runtime file %s imports modules/queue", file)
		}
	}
}

type testPayload struct {
	ID string
}

type describedProcessor struct {
	calls chan testPayload
}

func (p *describedProcessor) GestQueueProcessor() queue.ProcessorDefinition {
	return queue.ProcessorDefinition{
		Name: "DescribedProcessor",
		Processors: []queue.ProcessorBinding{
			{
				Queue:  "email",
				Handle: queue.Handle(p.Process),
			},
		},
	}
}

func (p *describedProcessor) Process(ctx context.Context, payload testPayload) error {
	_ = ctx
	p.calls <- payload
	return nil
}

func shutdownQueue(t *testing.T, q *queue.Queue) {
	t.Helper()
	if err := q.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}
}

func projectRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("find project root: %v", err)
	}
	return root
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}
