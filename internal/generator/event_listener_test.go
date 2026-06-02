package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateEventListenerMetadata(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.test/app\n\ngo 1.26.2\n")
	writeTestFile(t, root, "internal/users/users.listener.go", `package users

import "context"

type UserCreated struct {
	ID string
}

// @OnEvent("user.created")
type UserCreatedListener struct{}

func (l *UserCreatedListener) Handle(ctx context.Context, event UserCreated) error {
	return nil
}
`)

	packages, err := ScanPackages(root, ScanOptions{})
	if err != nil {
		t.Fatalf("ScanPackages returned error: %v", err)
	}
	listeners, diagnostics, err := ParseEventListeners(packages)
	if err != nil {
		t.Fatalf("ParseEventListeners returned error: %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diagnostics)
	}
	files, err := GenerateEventMetadataFiles(listeners)
	if err != nil {
		t.Fatalf("GenerateEventMetadataFiles returned error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("files = %d, want 1", len(files))
	}
	got := string(files[0].Content)
	for _, want := range []string{
		`import "github.com/r6m/gest/modules/events"`,
		`func (l *UserCreatedListener) GestEventListener() events.ListenerDefinition`,
		`Event:  "user.created"`,
		`Handle: events.Handle[UserCreated](l.Handle)`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated metadata missing %q:\n%s", want, got)
		}
	}
}

func TestParseEventListenerRejectsInvalidHandleSignature(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.test/app\n\ngo 1.26.2\n")
	writeTestFile(t, root, "internal/users/users.listener.go", `package users

import "context"

// @OnEvent("user.created")
type UserCreatedListener struct{}

func (l *UserCreatedListener) Handle(ctx context.Context) error {
	return nil
}
`)

	packages, err := ScanPackages(root, ScanOptions{})
	if err != nil {
		t.Fatalf("ScanPackages returned error: %v", err)
	}
	_, diagnostics, err := ParseEventListeners(packages)
	if err != nil {
		t.Fatalf("ParseEventListeners returned error: %v", err)
	}
	if len(diagnostics) != 1 || diagnostics[0].Code != DiagnosticInvalidHandlerSignature {
		t.Fatalf("diagnostics = %#v, want invalid handler signature", diagnostics)
	}
}

func writeTestFile(t *testing.T, root string, path string, content string) {
	t.Helper()
	target := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
}
