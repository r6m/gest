package gest

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

type lifecycleRecorder struct {
	events *[]string
	name   string
}

func (r *lifecycleRecorder) record(event string) {
	*r.events = append(*r.events, r.name+"."+event)
}

type lifecycleService struct {
	*lifecycleRecorder
}

func newLifecycleService(events *[]string) *lifecycleService {
	return &lifecycleService{lifecycleRecorder: &lifecycleRecorder{events: events, name: "service"}}
}

func (s *lifecycleService) OnModuleInit(context.Context) error {
	s.record("OnModuleInit")
	return nil
}

func (s *lifecycleService) OnApplicationBootstrap(context.Context) error {
	s.record("OnApplicationBootstrap")
	return nil
}

func (s *lifecycleService) BeforeApplicationShutdown(context.Context) error {
	s.record("BeforeApplicationShutdown")
	return nil
}

func (s *lifecycleService) OnModuleDestroy(context.Context) error {
	s.record("OnModuleDestroy")
	return nil
}

func (s *lifecycleService) OnApplicationShutdown(context.Context) error {
	s.record("OnApplicationShutdown")
	return nil
}

type lifecycleController struct {
	*lifecycleRecorder
	service *lifecycleService
}

func newLifecycleController(service *lifecycleService, events *[]string) *lifecycleController {
	return &lifecycleController{
		lifecycleRecorder: &lifecycleRecorder{events: events, name: "controller"},
		service:           service,
	}
}

func (c *lifecycleController) OnModuleInit(context.Context) error {
	c.record("OnModuleInit")
	return nil
}

func (c *lifecycleController) OnApplicationBootstrap(context.Context) error {
	c.record("OnApplicationBootstrap")
	return nil
}

func (c *lifecycleController) BeforeApplicationShutdown(context.Context) error {
	c.record("BeforeApplicationShutdown")
	return nil
}

func (c *lifecycleController) OnModuleDestroy(context.Context) error {
	c.record("OnModuleDestroy")
	return nil
}

func (c *lifecycleController) OnApplicationShutdown(context.Context) error {
	c.record("OnApplicationShutdown")
	return nil
}

func (c *lifecycleController) GestController() ControllerDefinition {
	return ControllerDefinition{
		Name:     "LifecycleController",
		BasePath: "/lifecycle",
		Routes: []RouteDefinition{
			{
				Name:    "Show",
				Method:  http.MethodGet,
				Path:    "/",
				Handler: emptyHandler,
			},
		},
	}
}

type lifecycleGlobalService struct {
	*lifecycleRecorder
}

func newLifecycleGlobalService(events *[]string) *lifecycleGlobalService {
	return &lifecycleGlobalService{lifecycleRecorder: &lifecycleRecorder{events: events, name: "global"}}
}

func (s *lifecycleGlobalService) OnModuleInit(context.Context) error {
	s.record("OnModuleInit")
	return nil
}

func (s *lifecycleGlobalService) OnApplicationBootstrap(context.Context) error {
	s.record("OnApplicationBootstrap")
	return nil
}

type lifecycleGlobalConsumer struct {
	*lifecycleRecorder
	global *lifecycleGlobalService
}

func newLifecycleGlobalConsumer(global *lifecycleGlobalService, events *[]string) *lifecycleGlobalConsumer {
	return &lifecycleGlobalConsumer{
		lifecycleRecorder: &lifecycleRecorder{events: events, name: "consumer"},
		global:            global,
	}
}

func (s *lifecycleGlobalConsumer) OnModuleInit(context.Context) error {
	s.record("OnModuleInit")
	return nil
}

func (s *lifecycleGlobalConsumer) OnApplicationBootstrap(context.Context) error {
	s.record("OnApplicationBootstrap")
	return nil
}

type lifecycleGlobalController struct {
	consumer *lifecycleGlobalConsumer
}

func newLifecycleGlobalController(consumer *lifecycleGlobalConsumer) *lifecycleGlobalController {
	return &lifecycleGlobalController{consumer: consumer}
}

func (c *lifecycleGlobalController) GestController() ControllerDefinition {
	return ControllerDefinition{
		Name:     "LifecycleGlobalController",
		BasePath: "/global-lifecycle",
		Routes: []RouteDefinition{
			{
				Name:    "Show",
				Method:  http.MethodGet,
				Path:    "/",
				Handler: emptyHandler,
			},
		},
	}
}

func TestLifecycleStartupHookOrderAcrossProvidersAndControllers(t *testing.T) {
	events := []string{}
	app := New(WithRouter(newFakeRouter()))
	app.Import(NewModule(ModuleConfig{
		Name: "LifecycleModule",
		Providers: Providers(
			Value(&events),
			Provide(newLifecycleService),
			Controller(newLifecycleController),
		),
	}))

	if err := app.bootstrap(); err != nil {
		t.Fatalf("bootstrap returned error: %v", err)
	}

	want := []string{
		"service.OnModuleInit",
		"controller.OnModuleInit",
		"service.OnApplicationBootstrap",
		"controller.OnApplicationBootstrap",
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %#v, want %#v", events, want)
	}
}

func TestLifecycleOrderWithGlobalProviderDependency(t *testing.T) {
	events := []string{}
	global := NewModule(ModuleConfig{
		Name:   "GlobalLifecycleModule",
		Global: true,
		Providers: Providers(
			Value(&events),
			Provide(newLifecycleGlobalService),
		),
	})
	feature := NewModule(ModuleConfig{
		Name: "FeatureLifecycleModule",
		Providers: Providers(
			Provide(newLifecycleGlobalConsumer),
			Controller(newLifecycleGlobalController),
		),
	})
	app := New(WithRouter(newFakeRouter()))
	app.Import(global, feature)

	if err := app.bootstrap(); err != nil {
		t.Fatalf("bootstrap returned error: %v", err)
	}

	want := []string{
		"global.OnModuleInit",
		"consumer.OnModuleInit",
		"global.OnApplicationBootstrap",
		"consumer.OnApplicationBootstrap",
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %#v, want %#v", events, want)
	}
}

func TestLifecycleBootstrapHookRunsAfterRouteRegistration(t *testing.T) {
	events := []string{}
	router := newFakeRouter()
	app := New(WithRouter(router))
	app.Import(NewModule(ModuleConfig{
		Name: "LifecycleModule",
		Providers: Providers(
			Value(&events),
			Provide(newLifecycleService),
			Controller(newLifecycleController),
		),
	}))

	if err := app.bootstrap(); err != nil {
		t.Fatalf("bootstrap returned error: %v", err)
	}
	if len(router.routes) != 1 {
		t.Fatalf("registered routes = %d, want 1", len(router.routes))
	}
	if events[len(events)-1] != "controller.OnApplicationBootstrap" {
		t.Fatalf("last event = %q, want bootstrap hook", events[len(events)-1])
	}
}

func TestLifecycleShutdownHookOrder(t *testing.T) {
	events := []string{}
	app := New(WithRouter(newFakeRouter()))
	app.Import(NewModule(ModuleConfig{
		Name: "LifecycleModule",
		Providers: Providers(
			Value(&events),
			Provide(newLifecycleService),
			Controller(newLifecycleController),
		),
	}))
	if err := app.bootstrap(); err != nil {
		t.Fatalf("bootstrap returned error: %v", err)
	}
	events = []string{}

	if err := app.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}

	want := []string{
		"controller.BeforeApplicationShutdown",
		"service.BeforeApplicationShutdown",
		"controller.OnModuleDestroy",
		"service.OnModuleDestroy",
		"controller.OnApplicationShutdown",
		"service.OnApplicationShutdown",
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %#v, want %#v", events, want)
	}
}

var errLifecycleInit = errors.New("init failed")

type failingLifecycleService struct{}

func newFailingLifecycleService() *failingLifecycleService {
	return &failingLifecycleService{}
}

func (s *failingLifecycleService) OnModuleInit(context.Context) error {
	return errLifecycleInit
}

type failingLifecycleController struct {
	service *failingLifecycleService
}

func newFailingLifecycleController(service *failingLifecycleService) *failingLifecycleController {
	return &failingLifecycleController{service: service}
}

func (c *failingLifecycleController) GestController() ControllerDefinition {
	return ControllerDefinition{
		Name: "FailingLifecycleController",
		Routes: []RouteDefinition{
			{
				Name:    "Show",
				Method:  http.MethodGet,
				Path:    "/failing",
				Handler: emptyHandler,
			},
		},
	}
}

func TestLifecycleHookErrorStopsStartup(t *testing.T) {
	router := newFakeRouter()
	app := New(WithRouter(router))
	app.Import(NewModule(ModuleConfig{
		Name: "FailingLifecycleModule",
		Providers: Providers(
			Provide(newFailingLifecycleService),
			Controller(newFailingLifecycleController),
		),
	}))

	err := app.bootstrap()
	if err == nil {
		t.Fatal("bootstrap returned nil error, want lifecycle error")
	}
	var lifecycleErr *lifecycleError
	if !errors.As(err, &lifecycleErr) {
		t.Fatalf("error type = %T, want *lifecycleError", err)
	}
	if lifecycleErr.Code != "LIFECYCLE_HOOK_FAILED" || lifecycleErr.Hook != "OnModuleInit" {
		t.Fatalf("lifecycle error = %#v, want OnModuleInit failure", lifecycleErr)
	}
	if !errors.Is(err, errLifecycleInit) {
		t.Fatalf("error = %v, want wrapped init error", err)
	}
	if len(router.routes) != 0 {
		t.Fatalf("registered routes = %d, want 0 after failed startup", len(router.routes))
	}
}

type unusedLifecycleService struct {
	events *[]string
}

func newUnusedLifecycleService(events *[]string) *unusedLifecycleService {
	return &unusedLifecycleService{events: events}
}

func (s *unusedLifecycleService) OnApplicationShutdown(context.Context) error {
	*s.events = append(*s.events, "unused.OnApplicationShutdown")
	return nil
}

func TestLifecycleShutdownCallsEagerProviders(t *testing.T) {
	events := []string{}
	app := New(WithRouter(newFakeRouter()))
	app.Import(NewModule(ModuleConfig{
		Name: "LifecycleModule",
		Providers: Providers(
			Value(&events),
			Provide(newUnusedLifecycleService),
			Provide(newLifecycleService),
			Controller(newLifecycleController),
		),
	}))
	if err := app.bootstrap(); err != nil {
		t.Fatalf("bootstrap returned error: %v", err)
	}
	events = []string{}

	if err := app.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}

	got := strings.Join(events, ",")
	if !strings.Contains(got, "unused.OnApplicationShutdown") {
		t.Fatalf("events = %#v, want unused eager provider shutdown hook", events)
	}
}
