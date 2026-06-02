package logger_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r6m/gest"
	"github.com/r6m/gest/modules/logger"
)

type loggingService struct {
	logger *slog.Logger
}

func newLoggingService(logger *slog.Logger) *loggingService {
	return &loggingService{logger: logger}
}

type simpleController struct{}

func newSimpleController() *simpleController {
	return &simpleController{}
}

func (c *simpleController) GestController() gest.ControllerDefinition {
	return gest.ControllerDefinition{
		Name: "SimpleController",
		Routes: []gest.RouteDefinition{
			{
				Method: http.MethodGet,
				Path:   "/",
				Handler: func(ctx *gest.Context) error {
					return ctx.NoContent(http.StatusNoContent)
				},
			},
		},
	}
}

func TestModuleRegistersSlogLogger(t *testing.T) {
	container := newContainer(t, logger.Module(logger.Options{Writer: &bytes.Buffer{}}))

	value, err := container.Resolve(gest.TokenOf[*slog.Logger]())
	if err != nil {
		t.Fatalf("Resolve *slog.Logger returned error: %v", err)
	}
	if _, ok := value.(*slog.Logger); !ok {
		t.Fatalf("resolved value = %T, want *slog.Logger", value)
	}
}

func TestServiceCanInjectSlogLogger(t *testing.T) {
	mod := gest.NewModule(gest.ModuleConfig{
		Name: "AppModule",
		Imports: gest.Imports(
			logger.Module(logger.Options{Writer: &bytes.Buffer{}}),
		),
		Providers: gest.Providers(
			gest.Provide(newLoggingService),
		),
	})
	container := newContainer(t, mod)

	value, err := container.Resolve(gest.TokenOf[*loggingService]())
	if err != nil {
		t.Fatalf("Resolve *loggingService returned error: %v", err)
	}
	service, ok := value.(*loggingService)
	if !ok {
		t.Fatalf("resolved value = %T, want *loggingService", value)
	}
	if service.logger == nil {
		t.Fatal("injected logger is nil")
	}
}

func TestTextFormatWritesExpectedOutput(t *testing.T) {
	var output bytes.Buffer
	log := newLogger(t, logger.Options{Format: "text", Writer: &output})

	log.Info("created", slog.String("id", "user-1"))

	got := output.String()
	if !strings.Contains(got, "level=INFO") || !strings.Contains(got, "msg=created") || !strings.Contains(got, "id=user-1") {
		t.Fatalf("text output = %q, want level, msg, and attr", got)
	}
}

func TestJSONFormatWritesExpectedOutput(t *testing.T) {
	var output bytes.Buffer
	log := newLogger(t, logger.Options{Format: "json", Writer: &output})

	log.Warn("created", slog.String("id", "user-1"))

	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatalf("Unmarshal output returned error: %v; output %q", err, output.String())
	}
	if entry["level"] != "WARN" || entry["msg"] != "created" || entry["id"] != "user-1" {
		t.Fatalf("json output = %#v, want level, msg, and attr", entry)
	}
}

func TestLevelFilteringWorks(t *testing.T) {
	var output bytes.Buffer
	log := newLogger(t, logger.Options{Level: "warn", Writer: &output})

	log.Info("hidden")
	log.Warn("visible")

	got := output.String()
	if strings.Contains(got, "hidden") {
		t.Fatalf("output = %q, want info log filtered", got)
	}
	if !strings.Contains(got, "visible") {
		t.Fatalf("output = %q, want warn log written", got)
	}
}

func TestInvalidLevelFails(t *testing.T) {
	container := newContainer(t, logger.Module(logger.Options{Level: "verbose"}))

	_, err := container.Resolve(gest.TokenOf[*slog.Logger]())
	if err == nil {
		t.Fatal("Resolve *slog.Logger returned nil error")
	}
	if !strings.Contains(err.Error(), "LOGGER_INVALID_LEVEL") || !strings.Contains(err.Error(), "verbose") {
		t.Fatalf("error = %q, want invalid level context", err.Error())
	}
}

func TestInvalidFormatFails(t *testing.T) {
	container := newContainer(t, logger.Module(logger.Options{Format: "yaml"}))

	_, err := container.Resolve(gest.TokenOf[*slog.Logger]())
	if err == nil {
		t.Fatal("Resolve *slog.Logger returned nil error")
	}
	if !strings.Contains(err.Error(), "LOGGER_INVALID_FORMAT") || !strings.Contains(err.Error(), "yaml") {
		t.Fatalf("error = %q, want invalid format context", err.Error())
	}
}

func TestAppWithoutLoggerModuleStillWorks(t *testing.T) {
	app := gest.New()
	app.Import(gest.NewModule(gest.ModuleConfig{
		Name:      "AppModule",
		Providers: gest.Providers(gest.Controller(newSimpleController)),
	}))

	recorder := httptest.NewRecorder()
	app.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}

func TestCoreRuntimeDoesNotImportLoggerModule(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	matches, err := filepath.Glob(filepath.Join(root, "*.go"))
	if err != nil {
		t.Fatalf("Glob returned error: %v", err)
	}
	for _, file := range matches {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("ReadFile %s returned error: %v", file, err)
		}
		if strings.Contains(string(content), "github.com/r6m/gest/modules/logger") {
			t.Fatalf("core runtime file %s imports modules/logger", file)
		}
	}
}

func newLogger(t *testing.T, options logger.Options) *slog.Logger {
	t.Helper()
	log, err := logger.NewLogger(options)
	if err != nil {
		t.Fatalf("NewLogger returned error: %v", err)
	}
	return log
}

func newContainer(t *testing.T, mod gest.Module) gest.Container {
	t.Helper()
	container, err := gest.NewContainer(mod)
	if err != nil {
		t.Fatalf("NewContainer returned error: %v", err)
	}
	return container
}
