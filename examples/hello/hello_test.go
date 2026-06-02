package hello_test

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGestGenerateDryRunWorksForHelloExample(t *testing.T) {
	root := repoRoot(t)
	command := exec.Command("go", "run", "../../cmd/gest", "generate", "--dry-run")
	command.Dir = filepath.Join(root, "examples", "hello")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		t.Fatalf("gest generate --dry-run failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "scanned packages:") {
		t.Fatalf("stdout = %q, want generate summary", stdout.String())
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot find test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
