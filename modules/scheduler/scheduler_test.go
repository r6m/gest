package scheduler_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/r6m/gest/modules/scheduler"
)

func TestSchedulerRunsEveryTaskAndStops(t *testing.T) {
	s := scheduler.NewScheduler()
	calls := make(chan struct{}, 2)
	if err := s.Add(scheduler.Task{
		Identity: "test.every",
		Every:    "10ms",
		Run: func(ctx context.Context) error {
			calls <- struct{}{}
			return nil
		},
	}); err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if err := s.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	select {
	case <-calls:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("task did not run")
	}
	if err := s.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}
}

func TestSchedulerShutdownStopsIntervalWorkers(t *testing.T) {
	s := scheduler.NewScheduler()
	calls := make(chan struct{}, 4)
	if err := s.Add(scheduler.Task{
		Identity: "test.every",
		Every:    "10ms",
		Run: func(ctx context.Context) error {
			calls <- struct{}{}
			return nil
		},
	}); err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if err := s.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	select {
	case <-calls:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("task did not run before shutdown")
	}
	if err := s.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}
	drain(calls)
	select {
	case <-calls:
		t.Fatal("task ran after shutdown")
	case <-time.After(40 * time.Millisecond):
	}
}

func TestSchedulerRejectsInvalidCron(t *testing.T) {
	s := scheduler.NewScheduler()
	err := s.Add(scheduler.Task{
		Identity: "bad.cron",
		Cron:     "not cron",
		Run: func(ctx context.Context) error {
			return nil
		},
	})
	if err == nil {
		t.Fatal("Add returned nil error for invalid cron")
	}
}

func TestRegisterTaskUsesGeneratedMetadata(t *testing.T) {
	s := scheduler.NewScheduler()
	task := &describedTask{}
	if err := scheduler.RegisterTask(s, task); err != nil {
		t.Fatalf("RegisterTask returned error: %v", err)
	}
	if err := s.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	defer func() {
		if err := s.Shutdown(context.Background()); err != nil {
			t.Fatalf("Shutdown returned error: %v", err)
		}
	}()
	select {
	case <-task.calls:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("registered task did not run")
	}
}

func TestSchedulerShutdownHonorsContext(t *testing.T) {
	s := scheduler.NewScheduler()
	if err := s.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := s.Shutdown(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("Shutdown error = %v, want nil or context.Canceled", err)
	}
}

type describedTask struct {
	calls chan struct{}
}

func (t *describedTask) GestScheduledTask() scheduler.TaskDefinition {
	if t.calls == nil {
		t.calls = make(chan struct{}, 1)
	}
	return scheduler.TaskDefinition{
		Name: "DescribedTask",
		Tasks: []scheduler.Task{
			{
				Identity: "described.every",
				Every:    "10ms",
				Run:      scheduler.Handle(t.Run),
			},
		},
	}
}

func (t *describedTask) Run(ctx context.Context) error {
	_ = ctx
	t.calls <- struct{}{}
	return nil
}

func drain(ch <-chan struct{}) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}
