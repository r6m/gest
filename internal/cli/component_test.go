package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateControllerCreatesControllerFile(t *testing.T) {
	root := moduleFixture(t, nil)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"g", "controller", "project/team", "--no-update-module"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	content := readFile(t, filepath.Join(root, "internal", "project", "team", "team.controller.go"))
	assertOutputContains(t, stdout.String(), "CREATE internal/project/team/team.controller.go", "SKIP parent module update")
	assertOutputContains(t, content,
		"package team",
		`// @Controller("/team")`,
		"type TeamController struct{}",
		"func NewTeamController() *TeamController",
	)
	testContent := readFile(t, filepath.Join(root, "internal", "project", "team", "team.controller_test.go"))
	assertOutputContains(t, stdout.String(), "CREATE internal/project/team/team.controller_test.go")
	assertOutputContains(t, testContent, "func TestNewTeamController")
}

func TestGenerateServiceCreatesServiceFile(t *testing.T) {
	root := moduleFixture(t, nil)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"g", "service", "project/team", "--no-update-module"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	content := readFile(t, filepath.Join(root, "internal", "project", "team", "team.service.go"))
	assertOutputContains(t, stdout.String(), "CREATE internal/project/team/team.service.go", "SKIP parent module update")
	assertOutputContains(t, content,
		"package team",
		"type TeamService struct{}",
		"func NewTeamService() *TeamService",
	)
	testContent := readFile(t, filepath.Join(root, "internal", "project", "team", "team.service_test.go"))
	assertOutputContains(t, stdout.String(), "CREATE internal/project/team/team.service_test.go")
	assertOutputContains(t, testContent, "func TestNewTeamService")
}

func TestGenerateControllerNoTestSkipsTestFile(t *testing.T) {
	root := moduleFixture(t, nil)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"g", "controller", "project/team", "--no-update-module", "--no-test"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	assertFileMissing(t, filepath.Join(root, "internal", "project", "team", "team.controller_test.go"))
	assertOutputExcludes(t, stdout.String(), "team.controller_test.go")
}

func TestGenerateControllerUpdatesModuleProviders(t *testing.T) {
	root := moduleFixture(t, map[string]string{
		"internal/project/team/team.module.go": moduleSource("team", "project.team"),
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"g", "controller", "project/team"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	module := readFile(t, filepath.Join(root, "internal", "project", "team", "team.module.go"))
	assertOutputContains(t, stdout.String(), "UPDATE internal/project/team/team.module.go")
	assertOutputContains(t, module,
		"Providers: gest.Providers(",
		"gest.Controller(NewTeamController),",
	)
	assertOutputExcludes(t, module, removedExportCall())
}

func TestGenerateServiceUpdatesModuleProviders(t *testing.T) {
	root := moduleFixture(t, map[string]string{
		"internal/project/team/team.module.go": moduleSource("team", "project.team"),
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"g", "service", "project/team"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	module := readFile(t, filepath.Join(root, "internal", "project", "team", "team.module.go"))
	assertOutputContains(t, stdout.String(), "UPDATE internal/project/team/team.module.go")
	assertOutputContains(t, module,
		"Providers: gest.Providers(",
		"gest.Provide(NewTeamService),",
	)
	assertOutputExcludes(t, module, removedExportCall())
}

func TestGenerateComponentDryRunWritesNothing(t *testing.T) {
	root := moduleFixture(t, map[string]string{
		"internal/project/team/team.module.go": moduleSource("team", "project.team"),
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"g", "controller", "project/team", "--dry-run"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	assertFileMissing(t, filepath.Join(root, "internal", "project", "team", "team.controller.go"))
	module := readFile(t, filepath.Join(root, "internal", "project", "team", "team.module.go"))
	if strings.Contains(module, "NewTeamController") {
		t.Fatalf("dry-run updated module:\n%s", module)
	}
	assertOutputContains(t, stdout.String(),
		"DRY-RUN CREATE internal/project/team/team.controller.go",
		"DRY-RUN UPDATE internal/project/team/team.module.go",
	)
}

func TestGenerateComponentForceOverwritesOnlyTargetFile(t *testing.T) {
	root := moduleFixture(t, map[string]string{
		"internal/project/team/team.service.go": "package team\n\nconst Old = true\n",
		"internal/project/team/keep.go":         "package team\n\nconst Keep = true\n",
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"g", "service", "project/team", "--force", "--no-update-module"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	service := readFile(t, filepath.Join(root, "internal", "project", "team", "team.service.go"))
	keep := readFile(t, filepath.Join(root, "internal", "project", "team", "keep.go"))
	if strings.Contains(service, "Old") {
		t.Fatalf("expected service file overwrite:\n%s", service)
	}
	if !strings.Contains(keep, "Keep") {
		t.Fatalf("expected unrelated file to remain:\n%s", keep)
	}
}

func TestGenerateComponentExistingFileWithoutForceErrors(t *testing.T) {
	root := moduleFixture(t, map[string]string{
		"internal/project/team/team.controller.go": "package team\n",
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"g", "controller", "project/team"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	assertOutputContains(t, stderr.String(), "already exists; use --force to overwrite")
}

func TestGenerateComponentMissingModuleWarns(t *testing.T) {
	root := moduleFixture(t, nil)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"g", "service", "project/team"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	assertOutputContains(t, stdout.String(),
		"WARN module file not found",
		"HINT add gest.Provide(NewTeamService) manually",
	)
}

func TestGenerateComponentsApplyGofmt(t *testing.T) {
	root := moduleFixture(t, nil)
	command := New()
	command.WorkDir = root

	code := command.Run(context.Background(), []string{"g", "controller", "project/team", "--no-update-module"}, ioDiscard{}, ioDiscard{})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	content := readFile(t, filepath.Join(root, "internal", "project", "team", "team.controller.go"))
	assertOutputContains(t, content, "func NewTeamController() *TeamController {\n\treturn &TeamController{}")
}

func moduleSource(packageName string, moduleName string) string {
	return `package ` + packageName + `

import "github.com/r6m/gest"

type Options struct{}

func Module(options Options) gest.Module {
	return gest.NewModule(gest.ModuleConfig{
		Name: "` + moduleName + `",
	})
}
`
}
