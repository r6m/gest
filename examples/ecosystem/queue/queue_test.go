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

func TestQueueExample(t *testing.T) {
	processed := make(chan emailJob, 1)
	app := gest.New()
	app.Import(gest.NewModule(gest.ModuleConfig{
		Name: "QueueExample",
		Imports: gest.Imports(
			queue.Module(queue.Options{}),
		),
		Providers: gest.Providers(
			gest.Value(processed, gest.As[chan emailJob]()),
			gest.Provide(newEmailProcessor),
			gest.Provide(newEmailService),
		),
	}))
	app.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	defer func() {
		if err := app.Shutdown(context.Background()); err != nil {
			t.Fatalf("Shutdown returned error: %v", err)
		}
	}()

	select {
	case got := <-processed:
		if got.To != "ada@example.test" {
			t.Fatalf("email recipient = %q, want ada@example.test", got.To)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("processor did not receive job")
	}
}

type emailJob struct {
	To string
}

type emailService struct {
	queue *queue.Queue
}

func newEmailService(q *queue.Queue) *emailService {
	return &emailService{queue: q}
}

func (s *emailService) OnApplicationBootstrap(ctx context.Context) error {
	return s.queue.Enqueue(ctx, "emails", emailJob{To: "ada@example.test"})
}

type emailProcessor struct {
	queue     *queue.Queue
	processed chan emailJob
}

func newEmailProcessor(q *queue.Queue, processed chan emailJob) *emailProcessor {
	return &emailProcessor{queue: q, processed: processed}
}

func (p *emailProcessor) OnModuleInit(ctx context.Context) error {
	_ = ctx
	return queue.RegisterProcessor(p.queue, p)
}

func (p *emailProcessor) GestQueueProcessor() queue.ProcessorDefinition {
	return queue.ProcessorDefinition{
		Name: "EmailProcessor",
		Processors: []queue.ProcessorBinding{
			{
				Queue:  "emails",
				Handle: queue.Handle(p.Process),
			},
		},
	}
}

func (p *emailProcessor) Process(ctx context.Context, job emailJob) error {
	_ = ctx
	p.processed <- job
	return nil
}
