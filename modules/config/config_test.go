package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/r6m/gest"
	"github.com/r6m/gest/modules/config"
)

type appConfig struct {
	Port      string        `env:"PORT" default:"3000"`
	JWTSecret string        `env:"JWT_SECRET" validate:"required"`
	Debug     bool          `env:"DEBUG"`
	Workers   int           `env:"WORKERS"`
	Limit     uint          `env:"LIMIT"`
	Ratio     float64       `env:"RATIO"`
	Timeout   time.Duration `env:"TIMEOUT"`
}

func TestServiceLoadsEnvFile(t *testing.T) {
	file := writeEnv(t, "app.env", "PORT=8080\nJWT_SECRET=secret\n")
	service := newService(t, config.Options{EnvFiles: []string{file}})

	if got := service.Get("PORT"); got != "8080" {
		t.Fatalf("PORT = %q, want 8080", got)
	}
}

func TestModuleIsGlobal(t *testing.T) {
	definition := config.Module(config.Options{}).Definition()

	if !definition.Global {
		t.Fatal("Global = false, want true")
	}
}

func TestLaterEnvFileOverridesEarlier(t *testing.T) {
	first := writeEnv(t, "first.env", "PORT=3000\n")
	second := writeEnv(t, "second.env", "PORT=4000\n")
	service := newService(t, config.Options{EnvFiles: []string{first, second}})

	if got := service.Get("PORT"); got != "4000" {
		t.Fatalf("PORT = %q, want 4000", got)
	}
}

func TestOSEnvOverridesFile(t *testing.T) {
	file := writeEnv(t, "app.env", "PORT=3000\n")
	t.Setenv("PORT", "9000")
	service := newService(t, config.Options{EnvFiles: []string{file}})

	if got := service.Get("PORT"); got != "9000" {
		t.Fatalf("PORT = %q, want 9000", got)
	}
}

func TestMissingOptionalFileDoesNotFail(t *testing.T) {
	service := newService(t, config.Options{EnvFiles: []string{filepath.Join(t.TempDir(), "missing.env")}})

	if service == nil {
		t.Fatal("service is nil")
	}
}

func TestRequiredMissingFileFails(t *testing.T) {
	_, err := config.NewService(config.Options{RequiredFiles: []string{filepath.Join(t.TempDir(), "missing.env")}})
	if err == nil {
		t.Fatal("NewService returned nil error")
	}
	if !strings.Contains(err.Error(), "CONFIG_REQUIRED_FILE_MISSING") {
		t.Fatalf("error = %q, want required file code", err.Error())
	}
}

func TestServiceGetters(t *testing.T) {
	file := writeEnv(t, "app.env", "PORT=8080\nDEBUG=true\nRATIO=1.5\n")
	service := newService(t, config.Options{EnvFiles: []string{file}})

	required, err := service.Required("PORT")
	if err != nil {
		t.Fatalf("Required returned error: %v", err)
	}
	if required != "8080" {
		t.Fatalf("Required PORT = %q, want 8080", required)
	}
	port, err := service.Int("PORT")
	if err != nil || port != 8080 {
		t.Fatalf("Int PORT = %d, %v, want 8080, nil", port, err)
	}
	debug, err := service.Bool("DEBUG")
	if err != nil || !debug {
		t.Fatalf("Bool DEBUG = %v, %v, want true, nil", debug, err)
	}
	ratio, err := service.Float("RATIO")
	if err != nil || ratio != 1.5 {
		t.Fatalf("Float RATIO = %v, %v, want 1.5, nil", ratio, err)
	}
	if _, err := service.Required("MISSING"); err == nil {
		t.Fatal("Required MISSING returned nil error")
	}
}

func TestStructLoadsAndResolvesThroughGestDI(t *testing.T) {
	file := writeEnv(t, "app.env", strings.Join([]string{
		"PORT=8080",
		"JWT_SECRET=secret",
		"DEBUG=true",
		"WORKERS=4",
		"LIMIT=12",
		"RATIO=2.5",
		"TIMEOUT=250ms",
		"QUOTED=\"hello world\"",
	}, "\n"))
	container := newContainer(t, config.Module(config.Options{
		EnvFiles: []string{file},
		Load: []config.LoadTarget{
			config.Struct[appConfig](),
		},
	}))

	value, err := container.Resolve(gest.TokenOf[*appConfig]())
	if err != nil {
		t.Fatalf("Resolve appConfig returned error: %v", err)
	}
	got, ok := value.(*appConfig)
	if !ok {
		t.Fatalf("resolved value = %T, want *appConfig", value)
	}
	if got.Port != "8080" || got.JWTSecret != "secret" || !got.Debug || got.Workers != 4 || got.Limit != 12 ||
		got.Ratio != 2.5 || got.Timeout != 250*time.Millisecond {
		t.Fatalf("app config = %#v, want values loaded from env", got)
	}
}

func TestStructDefaultTag(t *testing.T) {
	file := writeEnv(t, "app.env", "JWT_SECRET=secret\n")
	container := newContainer(t, config.Module(config.Options{
		EnvFiles: []string{file},
		Load:     []config.LoadTarget{config.Struct[appConfig]()},
	}))

	value, err := container.Resolve(gest.TokenOf[*appConfig]())
	if err != nil {
		t.Fatalf("Resolve appConfig returned error: %v", err)
	}
	got, ok := value.(*appConfig)
	if !ok {
		t.Fatalf("resolved value = %T, want *appConfig", value)
	}
	if got := got.Port; got != "3000" {
		t.Fatalf("Port = %q, want default 3000", got)
	}
}

func TestStructRequiredValidationTag(t *testing.T) {
	container := newContainer(t, config.Module(config.Options{
		Load: []config.LoadTarget{config.Struct[appConfig]()},
	}))

	_, err := container.Resolve(gest.TokenOf[*appConfig]())
	if err == nil {
		t.Fatal("Resolve appConfig returned nil error")
	}
	if !strings.Contains(err.Error(), "JWTSecret") || !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Fatalf("error = %q, want field and key context", err.Error())
	}
}

func TestStructConversionFailureIncludesFieldAndKeyContext(t *testing.T) {
	file := writeEnv(t, "app.env", "JWT_SECRET=secret\nWORKERS=many\n")
	container := newContainer(t, config.Module(config.Options{
		EnvFiles: []string{file},
		Load:     []config.LoadTarget{config.Struct[appConfig]()},
	}))

	_, err := container.Resolve(gest.TokenOf[*appConfig]())
	if err == nil {
		t.Fatal("Resolve appConfig returned nil error")
	}
	if !strings.Contains(err.Error(), "Workers") || !strings.Contains(err.Error(), "WORKERS") || !strings.Contains(err.Error(), "many") {
		t.Fatalf("error = %q, want conversion field/key/value context", err.Error())
	}
	var configErr *config.Error
	if !errors.As(err, &configErr) {
		t.Fatalf("error type = %T, want *config.Error", err)
	}
}

func TestModuleIsOptional(t *testing.T) {
	container := newContainer(t, gest.NewModule(gest.ModuleConfig{Name: "App"}))

	_, err := container.Resolve(gest.TokenOf[*config.Service]())
	if err == nil {
		t.Fatal("Resolve *config.Service returned nil error without importing config module")
	}
}

func TestCoreRuntimeDoesNotImportConfigModule(t *testing.T) {
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
		if strings.Contains(string(content), "github.com/r6m/gest/modules/config") {
			t.Fatalf("core runtime file %s imports modules/config", file)
		}
	}
}

func newService(t *testing.T, options config.Options) *config.Service {
	t.Helper()
	service, err := config.NewService(options)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}
	return service
}

func newContainer(t *testing.T, mod gest.Module) gest.Container {
	t.Helper()
	container, err := gest.NewContainer(mod)
	if err != nil {
		t.Fatalf("NewContainer returned error: %v", err)
	}
	return container
}

func writeEnv(t *testing.T, name string, content string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	return file
}
