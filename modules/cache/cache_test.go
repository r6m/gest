package cache_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/r6m/gest"
	"github.com/r6m/gest/modules/cache"
)

func TestCacheSetGetHitAndMiss(t *testing.T) {
	service := cache.NewService(cache.Options{})
	ctx := context.Background()

	if _, ok, err := service.Get(ctx, "missing"); err != nil || ok {
		t.Fatalf("Get missing = ok %v err %v, want miss without error", ok, err)
	}
	if err := service.Set(ctx, "key", []byte("value"), 0); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	value, ok, err := service.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !ok || string(value) != "value" {
		t.Fatalf("Get = %q ok %v, want value hit", value, ok)
	}
}

func TestCacheDeleteRemovesValue(t *testing.T) {
	service := cache.NewService(cache.Options{})
	ctx := context.Background()

	if err := service.Set(ctx, "key", []byte("value"), 0); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if err := service.Delete(ctx, "key"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, ok, err := service.Get(ctx, "key"); err != nil || ok {
		t.Fatalf("Get deleted = ok %v err %v, want miss without error", ok, err)
	}
}

func TestCacheTTLExpiresValue(t *testing.T) {
	service := cache.NewService(cache.Options{})
	ctx := context.Background()

	if err := service.Set(ctx, "key", []byte("value"), 10*time.Millisecond); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	time.Sleep(25 * time.Millisecond)
	if _, ok, err := service.Get(ctx, "key"); err != nil || ok {
		t.Fatalf("Get expired = ok %v err %v, want miss without error", ok, err)
	}
}

func TestCacheJSONHelpers(t *testing.T) {
	service := cache.NewService(cache.Options{})
	ctx := context.Background()
	want := cachedUser{ID: "u1", Name: "Ada"}

	if err := service.SetJSON(ctx, "user:u1", want, 0); err != nil {
		t.Fatalf("SetJSON returned error: %v", err)
	}
	var got cachedUser
	ok, err := service.GetJSON(ctx, "user:u1", &got)
	if err != nil {
		t.Fatalf("GetJSON returned error: %v", err)
	}
	if !ok || got != want {
		t.Fatalf("GetJSON = %#v ok %v, want %#v hit", got, ok, want)
	}
}

func TestCacheModuleGlobalInjection(t *testing.T) {
	root := gest.NewModule(gest.ModuleConfig{
		Name: "AppModule",
		Imports: gest.Imports(
			cache.Module(cache.Options{Global: true}),
		),
		Providers: gest.Providers(
			gest.Provide(newCacheConsumer),
		),
	})

	container, err := gest.NewContainer(root)
	if err != nil {
		t.Fatalf("NewContainer returned error: %v", err)
	}
	resolved, err := container.Resolve(gest.TokenOf[*cacheConsumer]())
	if err != nil {
		t.Fatalf("Resolve consumer returned error: %v", err)
	}
	consumer, ok := resolved.(*cacheConsumer)
	if !ok {
		t.Fatalf("consumer = %T, want *cacheConsumer", resolved)
	}
	if consumer.cache == nil {
		t.Fatal("consumer cache is nil")
	}
}

func TestCoreRuntimeDoesNotImportCacheModule(t *testing.T) {
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
		if strings.Contains(content, "github.com/r6m/gest/modules/cache") {
			t.Fatalf("core runtime file %s imports modules/cache", file)
		}
	}
}

type cachedUser struct {
	ID   string
	Name string
}

type cacheConsumer struct {
	cache *cache.Service
}

func newCacheConsumer(service *cache.Service) *cacheConsumer {
	return &cacheConsumer{cache: service}
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
