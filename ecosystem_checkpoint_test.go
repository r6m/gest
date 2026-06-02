package gest_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r6m/gest"
	"github.com/r6m/gest/modules/cache"
	"github.com/r6m/gest/modules/events"
	"github.com/r6m/gest/modules/queue"
	"github.com/r6m/gest/modules/scheduler"
)

func TestEcosystemModulesAreOptionalAndExplicit(t *testing.T) {
	assertMissingProvider[takesEventsBus](t, gest.NewModule(gest.ModuleConfig{
		Name:      "AppModule",
		Providers: gest.Providers(gest.Provide(newTakesEventsBus)),
	}))
	assertMissingProvider[takesCache](t, gest.NewModule(gest.ModuleConfig{
		Name:      "AppModule",
		Providers: gest.Providers(gest.Provide(newTakesCache)),
	}))
	assertMissingProvider[takesScheduler](t, gest.NewModule(gest.ModuleConfig{
		Name:      "AppModule",
		Providers: gest.Providers(gest.Provide(newTakesScheduler)),
	}))
	assertMissingProvider[takesQueue](t, gest.NewModule(gest.ModuleConfig{
		Name:      "AppModule",
		Providers: gest.Providers(gest.Provide(newTakesQueue)),
	}))
}

func TestEventsAndCacheGlobalModeRequiresExplicitImport(t *testing.T) {
	eventFeature := gest.NewModule(gest.ModuleConfig{
		Name:      "EventFeature",
		Providers: gest.Providers(gest.Provide(newTakesEventsBus)),
	})
	cacheFeature := gest.NewModule(gest.ModuleConfig{
		Name:      "CacheFeature",
		Providers: gest.Providers(gest.Provide(newTakesCache)),
	})

	assertMissingProvider[takesEventsBus](t, gest.NewModule(gest.ModuleConfig{
		Name:    "AppModule",
		Imports: gest.Imports(eventFeature),
	}))
	assertMissingProvider[takesCache](t, gest.NewModule(gest.ModuleConfig{
		Name:    "AppModule",
		Imports: gest.Imports(cacheFeature),
	}))
	assertMissingProvider[takesEventsBus](t, gest.NewModule(gest.ModuleConfig{
		Name: "AppModule",
		Imports: gest.Imports(
			events.Module(events.Options{}),
			eventFeature,
		),
	}))
	assertMissingProvider[takesCache](t, gest.NewModule(gest.ModuleConfig{
		Name: "AppModule",
		Imports: gest.Imports(
			cache.Module(cache.Options{}),
			cacheFeature,
		),
	}))

	assertResolveSucceeds[takesEventsBus](t, gest.NewModule(gest.ModuleConfig{
		Name: "AppModule",
		Imports: gest.Imports(
			events.Module(events.Options{Global: true}),
			eventFeature,
		),
	}))
	assertResolveSucceeds[takesCache](t, gest.NewModule(gest.ModuleConfig{
		Name: "AppModule",
		Imports: gest.Imports(
			cache.Module(cache.Options{Global: true}),
			cacheFeature,
		),
	}))
}

func TestCoreRuntimeDoesNotImportEcosystemModules(t *testing.T) {
	root := projectRoot(t)
	coreFiles := []string{
		"app.go",
		"binding.go",
		"container.go",
		"context.go",
		"controller.go",
		"module.go",
		"provider.go",
		"router.go",
		"stream.go",
	}
	for _, file := range coreFiles {
		content := readFile(t, filepath.Join(root, file))
		for _, importPath := range []string{
			"github.com/r6m/gest/modules/events",
			"github.com/r6m/gest/modules/scheduler",
			"github.com/r6m/gest/modules/cache",
			"github.com/r6m/gest/modules/queue",
			"github.com/r6m/gest/modules/websocket",
		} {
			if strings.Contains(content, importPath) {
				t.Fatalf("core runtime file %s imports %s", file, importPath)
			}
		}
	}

	websocketModule := filepath.Join(root, "modules", "websocket")
	if _, err := os.Stat(websocketModule); err == nil {
		t.Fatalf("modules/websocket exists before the WebSocket module phase")
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stat %s: %v", websocketModule, err)
	}
}

func assertMissingProvider[T any](t *testing.T, module gest.Module) {
	t.Helper()
	container, err := gest.NewContainer(module)
	if err != nil {
		t.Fatalf("NewContainer returned error: %v", err)
	}
	_, err = container.Resolve(gest.TokenOf[*T]())
	if err == nil {
		t.Fatalf("Resolve %T returned nil error, want missing provider", (*T)(nil))
	}
	if !strings.Contains(err.Error(), "DI_MISSING_PROVIDER") {
		t.Fatalf("Resolve error = %v, want DI_MISSING_PROVIDER", err)
	}
}

func assertResolveSucceeds[T any](t *testing.T, module gest.Module) {
	t.Helper()
	container, err := gest.NewContainer(module)
	if err != nil {
		t.Fatalf("NewContainer returned error: %v", err)
	}
	value, err := container.Resolve(gest.TokenOf[*T]())
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if value == nil {
		t.Fatal("Resolve returned nil value")
	}
}

type takesEventsBus struct {
	bus *events.Bus
}

func newTakesEventsBus(bus *events.Bus) *takesEventsBus {
	return &takesEventsBus{bus: bus}
}

type takesCache struct {
	cache *cache.Service
}

func newTakesCache(service *cache.Service) *takesCache {
	return &takesCache{cache: service}
}

type takesScheduler struct {
	scheduler *scheduler.Scheduler
}

func newTakesScheduler(scheduler *scheduler.Scheduler) *takesScheduler {
	return &takesScheduler{scheduler: scheduler}
}

type takesQueue struct {
	queue *queue.Queue
}

func newTakesQueue(q *queue.Queue) *takesQueue {
	return &takesQueue{queue: q}
}

func projectRoot(t *testing.T) string {
	t.Helper()
	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd returned error: %v", err)
	}
	return root
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			t.Fatalf("file %s does not exist", path)
		}
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}
