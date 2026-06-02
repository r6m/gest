package queue_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/r6m/gest"
	"github.com/r6m/gest/modules/queue"
)

func TestQueueModuleStartsRegisteredProcessorDuringAppBootstrap(t *testing.T) {
	calls := make(chan integrationPayload, 1)
	app := gest.New()
	app.Import(gest.NewModule(gest.ModuleConfig{
		Name: "AppModule",
		Imports: gest.Imports(
			queue.Module(queue.Options{}),
		),
		Providers: gest.Providers(
			gest.Value(calls, gest.As[chan integrationPayload]()),
			gest.Provide(newIntegrationProcessor),
			gest.Provide(newIntegrationProducer),
		),
	}))

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	app.ServeHTTP(response, request)
	defer func() {
		if err := app.Shutdown(context.Background()); err != nil {
			t.Fatalf("Shutdown returned error: %v", err)
		}
	}()

	select {
	case got := <-calls:
		if got.ID != "job-1" {
			t.Fatalf("payload ID = %q, want job-1", got.ID)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("processor did not receive job")
	}
}

type integrationPayload struct {
	ID string
}

type integrationProcessor struct {
	queue *queue.Queue
	calls chan integrationPayload
}

func newIntegrationProcessor(q *queue.Queue, calls chan integrationPayload) *integrationProcessor {
	return &integrationProcessor{queue: q, calls: calls}
}

func (p *integrationProcessor) OnModuleInit(ctx context.Context) error {
	_ = ctx
	return queue.RegisterProcessor(p.queue, p)
}

func (p *integrationProcessor) GestQueueProcessor() queue.ProcessorDefinition {
	return queue.ProcessorDefinition{
		Name: "IntegrationProcessor",
		Processors: []queue.ProcessorBinding{
			{
				Queue:  "integration",
				Handle: queue.Handle(p.Process),
			},
		},
	}
}

func (p *integrationProcessor) Process(ctx context.Context, payload integrationPayload) error {
	_ = ctx
	p.calls <- payload
	return nil
}

type integrationProducer struct {
	queue *queue.Queue
}

func newIntegrationProducer(q *queue.Queue) *integrationProducer {
	return &integrationProducer{queue: q}
}

func (p *integrationProducer) OnApplicationBootstrap(ctx context.Context) error {
	return p.queue.Enqueue(ctx, "integration", integrationPayload{ID: "job-1"})
}
