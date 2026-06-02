package cli

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewCreatesExpectedTree(t *testing.T) {
	root := fixtureWithFiles(t, nil)
	target := filepath.Join(root, "my-api")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"new", "my-api"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	for _, path := range newExpectedFiles() {
		assertFileExists(t, filepath.Join(target, path))
	}
	assertOutputContains(t, stdout.String(),
		"CREATE my-api/cmd/api/main.go",
		"CREATE my-api/internal/app/app.module.go",
		"CREATE my-api/internal/hello/hello.module.go",
		"CREATE my-api/internal/hello/hello.controller.go",
		"CREATE my-api/internal/hello/hello.dto.go",
		"CREATE my-api/internal/hello/hello_gest.gen.go",
		"CREATE my-api/gest.yaml",
		"CREATE my-api/go.mod",
		"CREATE my-api/go.sum",
		"RUN cd my-api",
		"RUN gest generate",
		"RUN gest build",
		"DONE created Gest app my-api",
	)
	assertGeneratedTreeExcludes(t, target, removedExportCall())
}

func TestNewGeneratedAppBuilds(t *testing.T) {
	target := generateNewAppFixture(t, []string{"new", "my-api"})

	runInDir(t, target, "go", "build", "./cmd/api")
}

func TestNewGeneratedAppRunsGoTest(t *testing.T) {
	target := generateNewAppFixture(t, []string{"new", "my-api"})

	runInDir(t, target, "go", "test", "./...")
}

func TestNewGeneratedAppRunsGestGenerate(t *testing.T) {
	target := generateNewAppFixture(t, []string{"new", "my-api"})
	command := New()
	command.WorkDir = target

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := command.Run(context.Background(), []string{"generate"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	assertOutputContains(t, stdout.String(), "scanned packages:", "generated: 0 updated: 0 skipped: 1")
}

func TestNewGeneratedAppRunsGestBuild(t *testing.T) {
	target := generateNewAppFixture(t, []string{"new", "my-api"})
	command := New()
	command.WorkDir = target

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := command.Run(context.Background(), []string{"build"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	assertOutputContains(t, stdout.String(),
		"generated: 0 updated: 0 skipped: 1",
		"go test ./...",
		"go build -trimpath -o bin/my-api ./cmd/api",
	)
	assertFileExists(t, filepath.Join(target, "bin", "my-api"))
}

func TestNewModuleOptionRewritesImports(t *testing.T) {
	root := fixtureWithFiles(t, nil)
	target := filepath.Join(root, "my-api")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"new", "my-api", "--module", "example.test/custom/api"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	assertOutputContains(t, readFile(t, filepath.Join(target, "go.mod")), "module example.test/custom/api")
	assertOutputContains(t, readFile(t, filepath.Join(target, "cmd", "api", "main.go")), `"example.test/custom/api/internal/app"`)
	assertOutputContains(t, readFile(t, filepath.Join(target, "internal", "app", "app.module.go")), `"example.test/custom/api/internal/hello"`)
}

func TestNewDryRunWritesNothing(t *testing.T) {
	root := fixtureWithFiles(t, nil)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"new", "my-api", "--dry-run"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	assertFileMissing(t, filepath.Join(root, "my-api"))
	assertOutputContains(t, stdout.String(), "DRY-RUN CREATE my-api/cmd/api/main.go", "DRY-RUN DONE created Gest app my-api")
}

func TestNewNonEmptyDirErrors(t *testing.T) {
	root := fixtureWithFiles(t, map[string]string{
		"my-api/keep.txt": "keep\n",
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"new", "my-api"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	assertOutputContains(t, stderr.String(), "is not empty", "--force")
	if stdout.String() != "" {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}
}

func TestNewForcePreservesUnrelatedFiles(t *testing.T) {
	root := fixtureWithFiles(t, map[string]string{
		"my-api/keep.txt":        "keep\n",
		"my-api/cmd/api/main.go": "package main\n\nconst Old = true\n",
	})
	target := filepath.Join(root, "my-api")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"new", "my-api", "--force"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	assertOutputContains(t, readFile(t, filepath.Join(target, "keep.txt")), "keep")
	mainSource := readFile(t, filepath.Join(target, "cmd", "api", "main.go"))
	assertOutputContains(t, mainSource, "server.Listen")
	assertOutputExcludes(t, mainSource, "Old")
}

func generateNewAppFixture(t *testing.T, args []string) string {
	t.Helper()

	root := fixtureWithFiles(t, nil)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), args, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("new app failed with code %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	return filepath.Join(root, "my-api")
}

func newExpectedFiles() []string {
	return []string{
		"cmd/api/main.go",
		"internal/app/app.module.go",
		"internal/hello/hello.module.go",
		"internal/hello/hello.controller.go",
		"internal/hello/hello.dto.go",
		"internal/hello/hello_gest.gen.go",
		"gest.yaml",
		"go.mod",
		"go.sum",
	}
}

func runInDir(t *testing.T, dir string, name string, args ...string) {
	t.Helper()

	command := exec.Command(name, args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, string(output))
	}
}

func assertGeneratedTreeExcludes(t *testing.T, root string, unexpected string) {
	t.Helper()

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(content), unexpected) {
			t.Fatalf("%s contains %q", path, unexpected)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk generated tree: %v", err)
	}
}
