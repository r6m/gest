package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildCommandBuildsTinyFixtureApp(t *testing.T) {
	root := buildFixture(t, map[string]string{})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"build"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	assertFileExists(t, filepath.Join(root, "bin", "app"))
	assertOutputContains(t, stdout.String(),
		"scanned packages: 1",
		"go build -trimpath -o bin/app ./cmd/api",
	)
}

func TestBuildCommandNoGenerateSkipsGenerator(t *testing.T) {
	root := buildFixture(t, map[string]string{
		"internal/bad/controller.go": invalidDecoratorSource(),
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"build", "--no-generate"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), "GEN_INVALID_DECORATOR_SYNTAX") {
		t.Fatalf("expected generator to be skipped, got stdout:\n%s", stdout.String())
	}
	assertOutputContains(t, stdout.String(), "go build -trimpath -o bin/app ./cmd/api")
}

func TestBuildCommandGenerateFailurePreventsBuild(t *testing.T) {
	root := buildFixture(t, map[string]string{
		"internal/bad/controller.go": invalidDecoratorSource(),
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"build"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	assertOutputContains(t, stdout.String(), "GEN_INVALID_DECORATOR_SYNTAX")
	if strings.Contains(stdout.String(), "go build") {
		t.Fatalf("expected build to be skipped, got stdout:\n%s", stdout.String())
	}
}

func TestBuildCommandTestFailurePreventsBuildWhenEnabled(t *testing.T) {
	root := buildFixture(t, map[string]string{
		"cmd/api/main_test.go": `package main

import "testing"

func TestFails(t *testing.T) {
	t.Fatal("boom")
}
`,
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"build", "--test"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	assertOutputContains(t, stdout.String(), "go test ./...")
	if strings.Contains(stdout.String(), "go build") {
		t.Fatalf("expected build to be skipped, got stdout:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String()+stderr.String(), "boom") {
		t.Fatalf("expected go test output to be visible, got stdout:\n%s\nstderr:\n%s", stdout.String(), stderr.String())
	}
}

func TestBuildCommandEntryAndOutFlagsOverrideConfig(t *testing.T) {
	root := buildFixture(t, map[string]string{
		"gest.yaml": "build:\n  entry: ./missing\n  output: bad/app\n",
		"cmd/worker/main.go": `package main

func main() {}
`,
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"build", "--entry", "./cmd/worker", "--out", "dist/worker"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	assertFileExists(t, filepath.Join(root, "dist", "worker"))
	assertOutputContains(t, stdout.String(), "go build -trimpath -o dist/worker ./cmd/worker")
}

func TestBuildCommandReturnsNonZeroOnBuildFailure(t *testing.T) {
	root := buildFixture(t, map[string]string{})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"build", "--entry", "./missing"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	assertOutputContains(t, stdout.String(), "go build -trimpath -o bin/app ./missing")
	if stderr.String() == "" {
		t.Fatal("expected go build error output to be visible")
	}
}

func TestBuildCommandNoTestOverridesConfig(t *testing.T) {
	root := buildFixture(t, map[string]string{
		"gest.yaml": "build:\n  test: true\n",
		"cmd/api/main_test.go": `package main

import "testing"

func TestFails(t *testing.T) {
	t.Fatal("boom")
}
`,
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"build", "--no-test"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), "go test") {
		t.Fatalf("expected tests to be skipped, got stdout:\n%s", stdout.String())
	}
	assertOutputContains(t, stdout.String(), "go build -trimpath -o bin/app ./cmd/api")
}

func buildFixture(t *testing.T, extra map[string]string) string {
	t.Helper()

	files := map[string]string{
		"go.mod": "module example.test/app\n\ngo 1.26.2\n",
		"cmd/api/main.go": `package main

func main() {}
`,
	}
	for path, content := range extra {
		files[path] = content
	}
	return fixtureWithFiles(t, files)
}

func invalidDecoratorSource() string {
	return `package bad

import "github.com/r6m/gest"

// @Controller(123)
type BadController struct{}

// @Get("/")
func (c *BadController) List(ctx *gest.Context) error {
	return nil
}
`
}

func TestBuildCommandRaceTagsAndLDFlagsArePrinted(t *testing.T) {
	root := buildFixture(t, map[string]string{})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"build", "--race", "--tags", "prod,sqlite", "--ldflags", "-s -w"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	assertOutputContains(t, stdout.String(), `go build -trimpath -race -tags prod,sqlite -ldflags "-s -w" -o bin/app ./cmd/api`)
}

func TestBuildCommandConfigCanDisableGenerate(t *testing.T) {
	root := buildFixture(t, map[string]string{
		"gest.yaml":                  "build:\n  generate: false\n",
		"internal/bad/controller.go": invalidDecoratorSource(),
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"build"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), "GEN_INVALID_DECORATOR_SYNTAX") {
		t.Fatalf("expected generator to be skipped, got stdout:\n%s", stdout.String())
	}
}

func TestBuildCommandCreatesOutputDirectory(t *testing.T) {
	root := buildFixture(t, map[string]string{})
	output := filepath.Join("nested", "bin", "api")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"build", "--out", output}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	assertFileExists(t, filepath.Join(root, output))
}

func TestBuildCommandDoesNotCreateOutputOnGenerateFailure(t *testing.T) {
	root := buildFixture(t, map[string]string{
		"internal/bad/controller.go": invalidDecoratorSource(),
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"build", "--out", "bin/fail"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	assertFileMissing(t, filepath.Join(root, "bin", "fail"))
}

func TestBuildCommandDoesNotLeaveUnexpectedFiles(t *testing.T) {
	root := buildFixture(t, map[string]string{})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"build", "--out", "bin/app"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	entries, err := os.ReadDir(filepath.Join(root, "bin"))
	if err != nil {
		t.Fatalf("read bin dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "app" {
		t.Fatalf("unexpected bin entries: %#v", entries)
	}
}
