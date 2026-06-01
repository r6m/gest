package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateModuleCreatesModuleFile(t *testing.T) {
	root := moduleFixture(t, nil)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"g", "module", "project/team", "--no-update-parent"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	path := filepath.Join(root, "internal", "project", "team", "team.module.go")
	content := readFile(t, path)
	assertOutputContains(t, stdout.String(), "CREATE internal/project/team/team.module.go", "SKIP parent module update")
	assertOutputContains(t, content,
		"package team",
		"type Options struct{}",
		"func Module(options Options) gest.Module",
		`Name: "project.team"`,
	)
}

func TestGenerateModuleDryRunWritesNothing(t *testing.T) {
	root := moduleFixture(t, map[string]string{
		"internal/project/project.module.go": parentModuleSource("project", "project"),
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"g", "module", "project/team", "--dry-run"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	assertFileMissing(t, filepath.Join(root, "internal", "project", "team", "team.module.go"))
	parent := readFile(t, filepath.Join(root, "internal", "project", "project.module.go"))
	if strings.Contains(parent, "team.Module") {
		t.Fatalf("dry-run updated parent:\n%s", parent)
	}
	assertOutputContains(t, stdout.String(),
		"DRY-RUN CREATE internal/project/team/team.module.go",
		"DRY-RUN UPDATE internal/project/project.module.go",
	)
}

func TestGenerateModuleExistingFileWithoutForceErrors(t *testing.T) {
	root := moduleFixture(t, map[string]string{
		"internal/project/team/team.module.go": "package team\n",
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"g", "module", "project/team"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	assertOutputContains(t, stderr.String(), "already exists; use --force to overwrite")
}

func TestGenerateModuleForceOverwritesOnlyGeneratedFile(t *testing.T) {
	root := moduleFixture(t, map[string]string{
		"internal/project/team/team.module.go":  "package team\n\nconst Old = true\n",
		"internal/project/team/keep.service.go": "package team\n\nconst Keep = true\n",
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"g", "module", "project/team", "--force", "--no-update-parent"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	moduleContent := readFile(t, filepath.Join(root, "internal", "project", "team", "team.module.go"))
	keepContent := readFile(t, filepath.Join(root, "internal", "project", "team", "keep.service.go"))
	if strings.Contains(moduleContent, "Old") {
		t.Fatalf("expected module file to be overwritten:\n%s", moduleContent)
	}
	if !strings.Contains(keepContent, "Keep") {
		t.Fatalf("expected unrelated file to be preserved:\n%s", keepContent)
	}
}

func TestGenerateModuleUpdatesParentModule(t *testing.T) {
	root := moduleFixture(t, map[string]string{
		"internal/project/project.module.go": parentModuleSource("project", "project"),
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"g", "module", "project/team"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	parent := readFile(t, filepath.Join(root, "internal", "project", "project.module.go"))
	assertOutputContains(t, stdout.String(), "UPDATE internal/project/project.module.go")
	assertOutputContains(t, parent,
		`"example.test/app/internal/project/team"`,
		"Imports: gest.Imports(",
		"team.Module(team.Options{}),",
	)
}

func TestGenerateModuleUpdatesRootAppWhenParentMissing(t *testing.T) {
	root := moduleFixture(t, map[string]string{
		"internal/app/app.module.go": parentModuleSource("app", "app"),
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"g", "module", "project/team"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	app := readFile(t, filepath.Join(root, "internal", "app", "app.module.go"))
	assertOutputContains(t, stdout.String(), "UPDATE internal/app/app.module.go")
	assertOutputContains(t, app, "team.Module(team.Options{})")
}

func TestGenerateModuleMissingParentWarns(t *testing.T) {
	root := moduleFixture(t, nil)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"g", "module", "project/team"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	assertOutputContains(t, stdout.String(),
		"WARN parent module not found",
		"HINT add team.Module(team.Options{}) manually",
	)
}

func TestGenerateModuleAppliesGofmt(t *testing.T) {
	root := moduleFixture(t, nil)
	command := New()
	command.WorkDir = root

	code := command.Run(context.Background(), []string{"g", "module", "project/team", "--no-update-parent"}, ioDiscard{}, ioDiscard{})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	content := readFile(t, filepath.Join(root, "internal", "project", "team", "team.module.go"))
	if strings.Contains(content, "\t\t\t") {
		t.Fatalf("expected gofmt output, got:\n%s", content)
	}
	assertOutputContains(t, content, "return gest.NewModule(gest.ModuleConfig{\n\t\tName: \"project.team\",")
}

func moduleFixture(t *testing.T, extra map[string]string) string {
	t.Helper()

	files := map[string]string{
		"go.mod": "module example.test/app\n\ngo 1.26.2\n",
	}
	for path, content := range extra {
		files[path] = content
	}
	return fixtureWithFiles(t, files)
}

func parentModuleSource(packageName string, moduleName string) string {
	return `package ` + packageName + `

import "github.com/r6m/gest"

func Module(options Options) gest.Module {
	return gest.NewModule(gest.ModuleConfig{
		Name: "` + moduleName + `",
	})
}

type Options struct{}
`
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}
