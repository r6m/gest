package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/r6m/gest"
)

// Options configures the optional scheduler module.
type Options struct{}

// Module returns a Gest module that provides a lifecycle-managed scheduler.
func Module(options Options) gest.Module {
	_ = options
	return gest.NewModule(gest.ModuleConfig{
		Name: "SchedulerModule",
		Providers: gest.Providers(
			gest.Provide(NewScheduler),
		),
	})
}

// Scheduler runs registered tasks according to cron expressions or fixed intervals.
type Scheduler struct {
	mu      sync.Mutex
	cron    *cron.Cron
	tasks   []scheduledTask
	started bool
	cancel  context.CancelFunc
	done    chan struct{}
	wg      sync.WaitGroup
}

// NewScheduler creates an empty scheduler.
func NewScheduler() *Scheduler {
	return &Scheduler{
		cron: cron.New(),
		done: make(chan struct{}),
	}
}

// Runner runs a scheduled task.
type Runner func(context.Context) error

// TaskDefinition describes generated task metadata.
type TaskDefinition struct {
	Name  string
	Tasks []Task
}

// Task describes one generated schedule entry.
type Task struct {
	Identity string
	Cron     string
	Every    string
	Run      Runner
}

// DescribedTask is implemented by generated task metadata.
type DescribedTask interface {
	GestScheduledTask() TaskDefinition
}

type scheduledTask struct {
	identity string
	every    time.Duration
	run      Runner
}

// RegisterTask registers generated metadata from a task provider.
func RegisterTask(scheduler *Scheduler, task DescribedTask) error {
	if scheduler == nil {
		return fmt.Errorf("SCHEDULER_INVALID_SCHEDULER: scheduler is nil")
	}
	if task == nil {
		return fmt.Errorf("SCHEDULER_INVALID_TASK: task is nil")
	}
	definition := task.GestScheduledTask()
	if definition.Name == "" {
		return fmt.Errorf("SCHEDULER_INVALID_TASK: task name is empty")
	}
	for _, entry := range definition.Tasks {
		if err := scheduler.Add(entry); err != nil {
			return err
		}
	}
	return nil
}

// Add registers one task schedule. It must be called before scheduler startup.
func (s *Scheduler) Add(task Task) error {
	if s == nil {
		return fmt.Errorf("SCHEDULER_INVALID_SCHEDULER: scheduler is nil")
	}
	if task.Identity == "" {
		return fmt.Errorf("SCHEDULER_INVALID_TASK: task identity is empty")
	}
	if task.Run == nil {
		return fmt.Errorf("SCHEDULER_INVALID_TASK: task %q runner is nil", task.Identity)
	}
	if task.Cron == "" && task.Every == "" {
		return fmt.Errorf("SCHEDULER_INVALID_TASK: task %q requires @Cron or @Every", task.Identity)
	}
	if task.Cron != "" && task.Every != "" {
		return fmt.Errorf("SCHEDULER_INVALID_TASK: task %q cannot use both @Cron and @Every", task.Identity)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return fmt.Errorf("SCHEDULER_ALREADY_STARTED: task %q cannot be registered after startup", task.Identity)
	}
	if task.Cron != "" {
		if _, err := s.cron.AddFunc(task.Cron, func() {
			_ = task.Run(context.Background())
		}); err != nil {
			return fmt.Errorf("SCHEDULER_INVALID_CRON: task %q: %w", task.Identity, err)
		}
		return nil
	}
	every, err := time.ParseDuration(task.Every)
	if err != nil {
		return fmt.Errorf("SCHEDULER_INVALID_EVERY: task %q: %w", task.Identity, err)
	}
	if every <= 0 {
		return fmt.Errorf("SCHEDULER_INVALID_EVERY: task %q duration must be positive", task.Identity)
	}
	s.tasks = append(s.tasks, scheduledTask{
		identity: task.Identity,
		every:    every,
		run:      task.Run,
	})
	return nil
}

// OnModuleInit starts the scheduler after task providers register their metadata.
func (s *Scheduler) OnModuleInit(ctx context.Context) error {
	_ = ctx
	return s.Start()
}

// BeforeApplicationShutdown stops running schedules.
func (s *Scheduler) BeforeApplicationShutdown(ctx context.Context) error {
	return s.Shutdown(ctx)
}

// Start begins cron and interval task delivery.
func (s *Scheduler) Start() error {
	if s == nil {
		return fmt.Errorf("SCHEDULER_INVALID_SCHEDULER: scheduler is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.done = make(chan struct{})
	s.started = true
	s.cron.Start()
	for _, task := range s.tasks {
		s.wg.Add(1)
		go runEvery(ctx, task, &s.wg)
	}
	go func() {
		<-ctx.Done()
		s.wg.Wait()
		close(s.done)
	}()
	return nil
}

// Shutdown stops all schedules.
func (s *Scheduler) Shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return nil
	}
	cancel := s.cancel
	done := s.done
	s.started = false
	s.cancel = nil
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	cronCtx := s.cron.Stop()
	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-cronCtx.Done():
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

func runEvery(ctx context.Context, task scheduledTask, wg *sync.WaitGroup) {
	defer wg.Done()
	ticker := time.NewTicker(task.every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = task.run(ctx)
		}
	}
}

// Handle adapts a task Run method to scheduler metadata.
func Handle(handler func(context.Context) error) Runner {
	return handler
}
