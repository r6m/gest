package scheduler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/r6m/gest"
	"github.com/r6m/gest/modules/scheduler"
)

func TestSchedulerModuleStartsRegisteredTaskDuringAppBootstrap(t *testing.T) {
	calls := make(chan struct{}, 1)
	app := gest.New()
	app.Import(gest.NewModule(gest.ModuleConfig{
		Name: "AppModule",
		Imports: gest.Imports(
			scheduler.Module(scheduler.Options{}),
		),
		Providers: gest.Providers(
			gest.Value(calls, gest.As[chan struct{}]()),
			gest.Provide(newIntegrationTask),
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
	case <-calls:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("scheduled task did not run")
	}
}

type integrationTask struct {
	scheduler *scheduler.Scheduler
	calls     chan struct{}
}

func newIntegrationTask(s *scheduler.Scheduler, calls chan struct{}) *integrationTask {
	return &integrationTask{scheduler: s, calls: calls}
}

func (t *integrationTask) OnModuleInit(ctx context.Context) error {
	_ = ctx
	return scheduler.RegisterTask(t.scheduler, t)
}

func (t *integrationTask) GestScheduledTask() scheduler.TaskDefinition {
	return scheduler.TaskDefinition{
		Name: "IntegrationTask",
		Tasks: []scheduler.Task{
			{
				Identity: "integration.every",
				Every:    "10ms",
				Run:      scheduler.Handle(t.Run),
			},
		},
	}
}

func (t *integrationTask) Run(ctx context.Context) error {
	_ = ctx
	t.calls <- struct{}{}
	return nil
}
