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

func TestSchedulerExample(t *testing.T) {
	ticks := make(chan string, 1)
	app := gest.New()
	app.Import(gest.NewModule(gest.ModuleConfig{
		Name: "SchedulerExample",
		Imports: gest.Imports(
			scheduler.Module(scheduler.Options{}),
		),
		Providers: gest.Providers(
			gest.Value(ticks, gest.As[chan string]()),
			gest.Provide(newHeartbeatTask),
		),
	}))
	app.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	defer func() {
		if err := app.Shutdown(context.Background()); err != nil {
			t.Fatalf("Shutdown returned error: %v", err)
		}
	}()

	select {
	case got := <-ticks:
		if got != "heartbeat" {
			t.Fatalf("tick = %q, want heartbeat", got)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("scheduled task did not run")
	}
}

type heartbeatTask struct {
	scheduler *scheduler.Scheduler
	ticks     chan string
}

func newHeartbeatTask(s *scheduler.Scheduler, ticks chan string) *heartbeatTask {
	return &heartbeatTask{scheduler: s, ticks: ticks}
}

func (t *heartbeatTask) OnModuleInit(ctx context.Context) error {
	_ = ctx
	return scheduler.RegisterTask(t.scheduler, t)
}

func (t *heartbeatTask) GestScheduledTask() scheduler.TaskDefinition {
	return scheduler.TaskDefinition{
		Name: "HeartbeatTask",
		Tasks: []scheduler.Task{
			{
				Identity: "heartbeat",
				Every:    "10ms",
				Run:      scheduler.Handle(t.Run),
			},
		},
	}
}

func (t *heartbeatTask) Run(ctx context.Context) error {
	_ = ctx
	t.ticks <- "heartbeat"
	return nil
}
