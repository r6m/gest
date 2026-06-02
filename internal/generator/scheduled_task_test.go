package generator

import (
	"strings"
	"testing"
)

func TestGenerateScheduledTaskMetadata(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.test/app\n\ngo 1.26.2\n")
	writeTestFile(t, root, "internal/jobs/jobs.task.go", `package jobs

import "context"

// @Cron("0 * * * *")
type CleanupTask struct{}

func (t *CleanupTask) Run(ctx context.Context) error {
	return nil
}

// @Every("5m")
type SyncTask struct{}

func (t *SyncTask) Run(ctx context.Context) error {
	return nil
}
`)

	packages, err := ScanPackages(root, ScanOptions{})
	if err != nil {
		t.Fatalf("ScanPackages returned error: %v", err)
	}
	tasks, diagnostics, err := ParseScheduledTasks(packages)
	if err != nil {
		t.Fatalf("ParseScheduledTasks returned error: %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diagnostics)
	}
	files, err := GenerateSchedulerMetadataFiles(tasks)
	if err != nil {
		t.Fatalf("GenerateSchedulerMetadataFiles returned error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("files = %d, want 1", len(files))
	}
	got := string(files[0].Content)
	for _, want := range []string{
		`import "github.com/r6m/gest/modules/scheduler"`,
		`func (t *CleanupTask) GestScheduledTask() scheduler.TaskDefinition`,
		`Identity: "0 * * * *"`,
		`Cron:     "0 * * * *"`,
		`Run:      scheduler.Handle(t.Run)`,
		`func (t *SyncTask) GestScheduledTask() scheduler.TaskDefinition`,
		`Identity: "5m"`,
		`Every:    "5m"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated metadata missing %q:\n%s", want, got)
		}
	}
}

func TestParseScheduledTaskRejectsInvalidRunSignature(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.test/app\n\ngo 1.26.2\n")
	writeTestFile(t, root, "internal/jobs/jobs.task.go", `package jobs

// @Every("5m")
type CleanupTask struct{}

func (t *CleanupTask) Run() error {
	return nil
}
`)

	packages, err := ScanPackages(root, ScanOptions{})
	if err != nil {
		t.Fatalf("ScanPackages returned error: %v", err)
	}
	_, diagnostics, err := ParseScheduledTasks(packages)
	if err != nil {
		t.Fatalf("ParseScheduledTasks returned error: %v", err)
	}
	if len(diagnostics) != 1 || diagnostics[0].Code != DiagnosticInvalidHandlerSignature {
		t.Fatalf("diagnostics = %#v, want invalid handler signature", diagnostics)
	}
}
