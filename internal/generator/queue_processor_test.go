package generator

import (
	"strings"
	"testing"
)

func TestGenerateQueueProcessorMetadata(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.test/app\n\ngo 1.26.2\n")
	writeTestFile(t, root, "internal/email/email.processor.go", `package email

import "context"

type EmailJob struct {
	ID string
}

// @Processor("email.send")
type EmailProcessor struct{}

func (p *EmailProcessor) Process(ctx context.Context, job EmailJob) error {
	return nil
}
`)

	packages, err := ScanPackages(root, ScanOptions{})
	if err != nil {
		t.Fatalf("ScanPackages returned error: %v", err)
	}
	processors, diagnostics, err := ParseQueueProcessors(packages)
	if err != nil {
		t.Fatalf("ParseQueueProcessors returned error: %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diagnostics)
	}
	files, err := GenerateQueueMetadataFiles(processors)
	if err != nil {
		t.Fatalf("GenerateQueueMetadataFiles returned error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("files = %d, want 1", len(files))
	}
	got := string(files[0].Content)
	for _, want := range []string{
		`import "github.com/r6m/gest/modules/queue"`,
		`func (p *EmailProcessor) GestQueueProcessor() queue.ProcessorDefinition`,
		`Queue:  "email.send"`,
		`Handle: queue.Handle[EmailJob](p.Process)`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated metadata missing %q:\n%s", want, got)
		}
	}
}

func TestParseQueueProcessorRejectsInvalidProcessSignature(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.test/app\n\ngo 1.26.2\n")
	writeTestFile(t, root, "internal/email/email.processor.go", `package email

// @Processor("email.send")
type EmailProcessor struct{}

func (p *EmailProcessor) Process() error {
	return nil
}
`)

	packages, err := ScanPackages(root, ScanOptions{})
	if err != nil {
		t.Fatalf("ScanPackages returned error: %v", err)
	}
	_, diagnostics, err := ParseQueueProcessors(packages)
	if err != nil {
		t.Fatalf("ParseQueueProcessors returned error: %v", err)
	}
	if len(diagnostics) != 1 || diagnostics[0].Code != DiagnosticInvalidHandlerSignature {
		t.Fatalf("diagnostics = %#v, want invalid handler signature", diagnostics)
	}
}
