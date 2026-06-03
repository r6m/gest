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

func TestGenerateListenerCreatesListenerFile(t *testing.T) {
	root := moduleFixture(t, nil)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"g", "listener", "project/team", "--no-update-module"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	content := readFile(t, filepath.Join(root, "internal", "project", "team", "team.listener.go"))
	assertOutputContains(t, stdout.String(), "CREATE internal/project/team/team.listener.go", "SKIP parent module update")
	assertOutputContains(t, content,
		"package team",
		`// @OnEvent("project.team.created")`,
		"type TeamListener struct",
		"func NewTeamListener(bus *events.Bus) *TeamListener",
		"func (l *TeamListener) OnModuleInit(ctx context.Context) error",
		"func (l *TeamListener) Handle(ctx context.Context, event TeamEvent) error",
	)
	testContent := readFile(t, filepath.Join(root, "internal", "project", "team", "team.listener_test.go"))
	assertOutputContains(t, stdout.String(), "CREATE internal/project/team/team.listener_test.go")
	assertOutputContains(t, testContent, "func TestNewTeamListener")
}

func TestGenerateTaskCreatesTaskFile(t *testing.T) {
	root := moduleFixture(t, nil)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"g", "task", "project/team", "--no-update-module"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	content := readFile(t, filepath.Join(root, "internal", "project", "team", "team.task.go"))
	assertOutputContains(t, stdout.String(), "CREATE internal/project/team/team.task.go", "SKIP parent module update")
	assertOutputContains(t, content,
		"package team",
		`// @Every("1m")`,
		"type TeamTask struct",
		"func NewTeamTask(scheduler *scheduler.Scheduler) *TeamTask",
		"func (t *TeamTask) OnModuleInit(ctx context.Context) error",
		"func (t *TeamTask) Run(ctx context.Context) error",
	)
	testContent := readFile(t, filepath.Join(root, "internal", "project", "team", "team.task_test.go"))
	assertOutputContains(t, stdout.String(), "CREATE internal/project/team/team.task_test.go")
	assertOutputContains(t, testContent, "func TestNewTeamTask")
}

func TestGenerateProcessorCreatesProcessorFile(t *testing.T) {
	root := moduleFixture(t, nil)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"g", "processor", "project/team", "--no-update-module"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	content := readFile(t, filepath.Join(root, "internal", "project", "team", "team.processor.go"))
	assertOutputContains(t, stdout.String(), "CREATE internal/project/team/team.processor.go", "SKIP parent module update")
	assertOutputContains(t, content,
		"package team",
		`// @Processor("project.team")`,
		"type TeamProcessor struct",
		"func NewTeamProcessor(queue *queue.Queue) *TeamProcessor",
		"func (p *TeamProcessor) OnModuleInit(ctx context.Context) error",
		"func (p *TeamProcessor) Process(ctx context.Context, job TeamJob) error",
	)
	testContent := readFile(t, filepath.Join(root, "internal", "project", "team", "team.processor_test.go"))
	assertOutputContains(t, stdout.String(), "CREATE internal/project/team/team.processor_test.go")
	assertOutputContains(t, testContent, "func TestNewTeamProcessor")
}

func TestGenerateGatewayCreatesNestedGatewayAndTestFiles(t *testing.T) {
	root := moduleFixture(t, nil)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"g", "gateway", "project/team", "--no-update-module"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	content := readFile(t, filepath.Join(root, "internal", "project", "team", "team.gateway.go"))
	assertOutputContains(t, stdout.String(), "CREATE internal/project/team/team.gateway.go", "SKIP parent module update")
	assertOutputContains(t, content,
		"package team",
		`// @Gateway("/ws/team")`,
		"type TeamGateway struct{}",
		"func NewTeamGateway() *TeamGateway",
		`// @Subscribe("project.team.message")`,
		"func (g *TeamGateway) HandleMessage(ctx context.Context, client *websocket.Client, msg TeamMessage) error",
	)
	for _, unexpected := range []string{"auth", "database", "queue", "events"} {
		assertOutputExcludes(t, content, unexpected)
	}
	testContent := readFile(t, filepath.Join(root, "internal", "project", "team", "team.gateway_test.go"))
	assertOutputContains(t, stdout.String(), "CREATE internal/project/team/team.gateway_test.go")
	assertOutputContains(t, testContent, "func TestNewTeamGateway")
}

func TestGenerateTaskNoTestSkipsTestFile(t *testing.T) {
	root := moduleFixture(t, nil)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"g", "task", "project/team", "--no-update-module", "--no-test"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	assertFileMissing(t, filepath.Join(root, "internal", "project", "team", "team.task_test.go"))
	assertOutputExcludes(t, stdout.String(), "team.task_test.go")
}

func TestGenerateProcessorNoTestSkipsTestFile(t *testing.T) {
	root := moduleFixture(t, nil)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"g", "processor", "project/team", "--no-update-module", "--no-test"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	assertFileMissing(t, filepath.Join(root, "internal", "project", "team", "team.processor_test.go"))
	assertOutputExcludes(t, stdout.String(), "team.processor_test.go")
}

func TestGenerateGatewayNoTestSkipsTestFile(t *testing.T) {
	root := moduleFixture(t, nil)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"g", "gateway", "project/team", "--no-update-module", "--no-test"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	assertFileMissing(t, filepath.Join(root, "internal", "project", "team", "team.gateway_test.go"))
	assertOutputExcludes(t, stdout.String(), "team.gateway_test.go")
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

func TestGenerateListenerUpdatesModuleProviders(t *testing.T) {
	root := moduleFixture(t, map[string]string{
		"internal/project/team/team.module.go": moduleSource("team", "project.team"),
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"g", "listener", "project/team"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	module := readFile(t, filepath.Join(root, "internal", "project", "team", "team.module.go"))
	assertOutputContains(t, stdout.String(), "UPDATE internal/project/team/team.module.go")
	assertOutputContains(t, module,
		"Providers: gest.Providers(",
		"gest.Provide(NewTeamListener),",
	)
}

func TestGenerateTaskUpdatesModuleProviders(t *testing.T) {
	root := moduleFixture(t, map[string]string{
		"internal/project/team/team.module.go": moduleSource("team", "project.team"),
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"g", "task", "project/team"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	module := readFile(t, filepath.Join(root, "internal", "project", "team", "team.module.go"))
	assertOutputContains(t, stdout.String(), "UPDATE internal/project/team/team.module.go")
	assertOutputContains(t, module,
		"Providers: gest.Providers(",
		"gest.Provide(NewTeamTask),",
	)
}

func TestGenerateProcessorUpdatesModuleProviders(t *testing.T) {
	root := moduleFixture(t, map[string]string{
		"internal/project/team/team.module.go": moduleSource("team", "project.team"),
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"g", "processor", "project/team"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	module := readFile(t, filepath.Join(root, "internal", "project", "team", "team.module.go"))
	assertOutputContains(t, stdout.String(), "UPDATE internal/project/team/team.module.go")
	assertOutputContains(t, module,
		"Providers: gest.Providers(",
		"gest.Provide(NewTeamProcessor),",
	)
}

func TestGenerateGatewayUpdatesModuleProviders(t *testing.T) {
	root := moduleFixture(t, map[string]string{
		"internal/project/team/team.module.go": moduleSource("team", "project.team"),
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"g", "gateway", "project/team"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	module := readFile(t, filepath.Join(root, "internal", "project", "team", "team.module.go"))
	assertOutputContains(t, stdout.String(), "UPDATE internal/project/team/team.module.go")
	assertOutputContains(t, module,
		`"github.com/r6m/gest/modules/websocket"`,
		"Providers: gest.Providers(",
		"websocket.Gateway(NewTeamGateway),",
	)
}

func TestGenerateGatewayNoUpdateModuleSkipsModule(t *testing.T) {
	root := moduleFixture(t, map[string]string{
		"internal/project/team/team.module.go": moduleSource("team", "project.team"),
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"g", "gateway", "project/team", "--no-update-module"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	module := readFile(t, filepath.Join(root, "internal", "project", "team", "team.module.go"))
	assertOutputContains(t, stdout.String(), "SKIP parent module update")
	assertOutputExcludes(t, module, "NewTeamGateway")
	assertOutputExcludes(t, module, "modules/websocket")
}

func TestGenerateProcessorDryRunWritesNothing(t *testing.T) {
	root := moduleFixture(t, map[string]string{
		"internal/project/team/team.module.go": moduleSource("team", "project.team"),
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"g", "processor", "project/team", "--dry-run"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	assertFileMissing(t, filepath.Join(root, "internal", "project", "team", "team.processor.go"))
	module := readFile(t, filepath.Join(root, "internal", "project", "team", "team.module.go"))
	if strings.Contains(module, "NewTeamProcessor") {
		t.Fatalf("dry-run updated module:\n%s", module)
	}
	assertOutputContains(t, stdout.String(),
		"DRY-RUN CREATE internal/project/team/team.processor.go",
		"DRY-RUN UPDATE internal/project/team/team.module.go",
	)
}

func TestGenerateGatewayDryRunWritesNothing(t *testing.T) {
	root := moduleFixture(t, map[string]string{
		"internal/project/team/team.module.go": moduleSource("team", "project.team"),
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"g", "gateway", "project/team", "--dry-run"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	assertFileMissing(t, filepath.Join(root, "internal", "project", "team", "team.gateway.go"))
	module := readFile(t, filepath.Join(root, "internal", "project", "team", "team.module.go"))
	assertOutputExcludes(t, module, "NewTeamGateway")
	assertOutputExcludes(t, module, "modules/websocket")
	assertOutputContains(t, stdout.String(),
		"DRY-RUN CREATE internal/project/team/team.gateway.go",
		"DRY-RUN UPDATE internal/project/team/team.module.go",
	)
}

func TestGenerateProcessorForceOverwritesTargetFile(t *testing.T) {
	root := moduleFixture(t, map[string]string{
		"internal/project/team/team.processor.go": "package team\n\nconst Old = true\n",
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"g", "processor", "project/team", "--force", "--no-update-module"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	content := readFile(t, filepath.Join(root, "internal", "project", "team", "team.processor.go"))
	if strings.Contains(content, "Old") {
		t.Fatalf("expected processor file overwrite:\n%s", content)
	}
}

func TestGenerateGatewayForceOverwritesTargetFile(t *testing.T) {
	root := moduleFixture(t, map[string]string{
		"internal/project/team/team.gateway.go": "package team\n\nconst Old = true\n",
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"g", "gateway", "project/team", "--force", "--no-update-module"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	content := readFile(t, filepath.Join(root, "internal", "project", "team", "team.gateway.go"))
	assertOutputExcludes(t, content, "Old")
	assertOutputContains(t, content, "type TeamGateway struct{}")
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

func TestGenerateGatewayAppliesGofmt(t *testing.T) {
	root := moduleFixture(t, nil)
	command := New()
	command.WorkDir = root

	code := command.Run(context.Background(), []string{"g", "gateway", "project/team", "--no-update-module"}, ioDiscard{}, ioDiscard{})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	content := readFile(t, filepath.Join(root, "internal", "project", "team", "team.gateway.go"))
	assertOutputContains(t, content, "func NewTeamGateway() *TeamGateway {\n\treturn &TeamGateway{}")
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
