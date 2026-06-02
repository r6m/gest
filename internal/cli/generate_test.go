package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateCommandWritesFile(t *testing.T) {
	root := generateFixture(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := New().Run(context.Background(), []string{"generate", "--root", root}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	assertFileExists(t, filepath.Join(root, "users", "users_gest.gen.go"))
	assertOutputContains(t, stdout.String(),
		"scanned packages: 1",
		"generated: 1 updated: 0 skipped: 0",
	)
}

func TestGenerateCommandSecondRunReportsSkipped(t *testing.T) {
	root := generateFixture(t)

	firstCode := New().Run(context.Background(), []string{"generate", "--root", root}, ioDiscard{}, ioDiscard{})
	if firstCode != 0 {
		t.Fatalf("expected first exit code 0, got %d", firstCode)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	secondCode := New().Run(context.Background(), []string{"generate", "--root", root}, &stdout, &stderr)

	if secondCode != 0 {
		t.Fatalf("expected second exit code 0, got %d stderr=%q", secondCode, stderr.String())
	}
	assertOutputContains(t, stdout.String(),
		"scanned packages: 1",
		"generated: 0 updated: 0 skipped: 1",
	)
}

func TestGenerateCommandDryRunDoesNotWrite(t *testing.T) {
	root := generateFixture(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := New().Run(context.Background(), []string{"generate", "--root", root, "--dry-run"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	assertFileMissing(t, filepath.Join(root, "users", "users_gest.gen.go"))
	assertOutputContains(t, stdout.String(),
		"scanned packages: 1",
		"dry-run generated: 1 updated: 0 skipped: 0",
	)
}

func TestGenerateCommandInvalidDecoratorExitsNonZero(t *testing.T) {
	root := fixtureWithFiles(t, map[string]string{
		"go.mod": "module example.test/app\n\ngo 1.26.2\n",
		"users/controller.go": `package users

import "github.com/r6m/gest"

// @Controller(123)
type UserController struct{}

// @Get("/")
func (c *UserController) List(ctx *gest.Context) error {
	return nil
}
`,
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := New().Run(context.Background(), []string{"generate", "--root", root}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	assertOutputContains(t, stdout.String(),
		"scanned packages: 1",
		"generated: 0 updated: 0 skipped: 0",
		"users/controller.go:5: GEN_INVALID_DECORATOR_SYNTAX",
		"hint: use @Controller(\"/users\")",
	)
	if got := stderr.String(); got != "error: generation failed\n" {
		t.Fatalf("unexpected stderr: %q", got)
	}
}

func TestGenerateCommandHonorsConfigRoot(t *testing.T) {
	workDir := fixtureWithFiles(t, map[string]string{
		"gest.yaml":               "generate:\n  root: ./app\n",
		"app/go.mod":              "module example.test/app\n\ngo 1.26.2\n",
		"app/users/controller.go": validControllerSource(),
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = workDir
	code := command.Run(context.Background(), []string{"generate"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	assertFileExists(t, filepath.Join(workDir, "app", "users", "users_gest.gen.go"))
	assertOutputContains(t, stdout.String(),
		"scanned packages: 1",
		"generated: 1 updated: 0 skipped: 0",
	)
}

func generateFixture(t *testing.T) string {
	t.Helper()

	return fixtureWithFiles(t, map[string]string{
		"go.mod":              "module example.test/app\n\ngo 1.26.2\n",
		"users/controller.go": validControllerSource(),
	})
}

func validControllerSource() string {
	return `package users

import "github.com/r6m/gest"

// @Controller("/users")
type UserController struct{}

// @Get("/")
func (c *UserController) List(ctx *gest.Context) error {
	return nil
}
`
}

func fixtureWithFiles(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create fixture dir: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write fixture file: %v", err)
		}
	}
	return root
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file %s to exist: %v", path, err)
	}
}

func assertFileMissing(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected file %s to be missing, stat err=%v", path, err)
	}
}

func assertOutputContains(t *testing.T, output string, wants ...string) {
	t.Helper()

	for _, want := range wants {
		if !strings.Contains(output, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, output)
		}
	}
}

func assertOutputExcludes(t *testing.T, output string, unexpected string) {
	t.Helper()

	if strings.Contains(output, unexpected) {
		t.Fatalf("expected output not to contain %q, got:\n%s", unexpected, output)
	}
}

func removedExportCall() string {
	return "gest." + "Export()"
}
