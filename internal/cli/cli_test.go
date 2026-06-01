package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRootHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := New().Run(context.Background(), nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if stderr.String() != "" {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}

	output := stdout.String()
	for _, want := range []string{
		"Usage:",
		"gest generate",
		"gest build",
		"gest g module",
		"gest g controller",
		"gest g service",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected help to contain %q, got:\n%s", want, output)
		}
	}
}

func TestUnknownCommandReturnsError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := New().Run(context.Background(), []string{"missing"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if stdout.String() != "" {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}
	if got := stderr.String(); got != "error: unknown command \"missing\"\n" {
		t.Fatalf("unexpected stderr: %q", got)
	}
}

func TestCommandsRouteToHandlers(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "generate", args: []string{"generate"}, want: "generate"},
		{name: "build", args: []string{"build"}, want: "build"},
		{name: "g module", args: []string{"g", "module"}, want: "g module"},
		{name: "g controller", args: []string{"g", "controller"}, want: "g controller"},
		{name: "g service", args: []string{"g", "service"}, want: "g service"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var called []string
			command := &CLI{
				Generate:           recordHandler(&called, "generate"),
				Build:              recordHandler(&called, "build"),
				GenerateModule:     recordHandler(&called, "g module"),
				GenerateController: recordHandler(&called, "g controller"),
				GenerateService:    recordHandler(&called, "g service"),
			}

			code := command.Run(context.Background(), tt.args, ioDiscard{}, ioDiscard{})

			if code != 0 {
				t.Fatalf("expected exit code 0, got %d", code)
			}
			if len(called) != 1 || called[0] != tt.want {
				t.Fatalf("expected handler %q, got %#v", tt.want, called)
			}
		})
	}
}

func TestHandlerErrorsAreReturnedAsNonZero(t *testing.T) {
	var stderr bytes.Buffer
	command := &CLI{
		Generate: func(context.Context) error {
			return errors.New("failed")
		},
	}

	code := command.Run(context.Background(), []string{"generate"}, ioDiscard{}, &stderr)

	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if got := stderr.String(); got != "error: failed\n" {
		t.Fatalf("unexpected stderr: %q", got)
	}
}

func TestRuntimePackagesDoNotImportCLIOrTooling(t *testing.T) {
	root := projectRoot(t)
	for _, file := range runtimeGoFiles(t, root) {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}

		for _, forbidden := range []string{
			"\"github.com/r6m/gest/internal/cli\"",
			"\"github.com/r6m/gest/internal/generator\"",
			"\"github.com/r6m/gest/internal/config\"",
		} {
			if strings.Contains(string(content), forbidden) {
				t.Fatalf("runtime file %s imports forbidden tooling package %s", file, forbidden)
			}
		}
	}
}

func recordHandler(called *[]string, name string) Handler {
	return func(context.Context) error {
		*called = append(*called, name)
		return nil
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) {
	return len(p), nil
}

func projectRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot find test file path")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func runtimeGoFiles(t *testing.T, root string) []string {
	t.Helper()

	var files []string
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read project root: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" {
			continue
		}
		if strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		files = append(files, filepath.Join(root, entry.Name()))
	}

	return files
}
