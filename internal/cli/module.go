package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

func (c *CLI) runGenerateModule(ctx context.Context, args []string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	options, err := parseModuleOptions(args)
	if err != nil {
		return err
	}
	modulePath, err := parseGeneratorPath(options.path)
	if err != nil {
		return err
	}

	result, err := c.generateModule(modulePath, options)
	if err != nil {
		return err
	}
	return writeModuleOutput(c.Stdout, result)
}

func (c *CLI) runGenerateController(ctx context.Context, args []string) error {
	return c.runGenerateComponent(ctx, args, componentController)
}

func (c *CLI) runGenerateService(ctx context.Context, args []string) error {
	return c.runGenerateComponent(ctx, args, componentService)
}

func (c *CLI) runGenerateListener(ctx context.Context, args []string) error {
	return c.runGenerateComponent(ctx, args, componentListener)
}

func (c *CLI) runGenerateTask(ctx context.Context, args []string) error {
	return c.runGenerateComponent(ctx, args, componentTask)
}

func (c *CLI) runGenerateProcessor(ctx context.Context, args []string) error {
	return c.runGenerateComponent(ctx, args, componentProcessor)
}

func (c *CLI) runGenerateGateway(ctx context.Context, args []string) error {
	return c.runGenerateComponent(ctx, args, componentGateway)
}

func (c *CLI) runGenerateResource(ctx context.Context, args []string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	options, err := parseResourceOptions(args)
	if err != nil {
		return err
	}
	resourcePath, err := parseGeneratorPath(options.path)
	if err != nil {
		return err
	}

	result, err := c.generateResource(resourcePath, options)
	if err != nil {
		return err
	}
	return writeModuleOutput(c.Stdout, result)
}

type moduleOptions struct {
	path         string
	dryRun       bool
	force        bool
	updateParent bool
}

func parseModuleOptions(args []string) (moduleOptions, error) {
	options := moduleOptions{updateParent: true}
	paths := make([]string, 0, 1)
	for _, arg := range args {
		switch arg {
		case "--dry-run":
			options.dryRun = true
		case "--force":
			options.force = true
		case "--no-update-parent":
			options.updateParent = false
		default:
			if strings.HasPrefix(arg, "-") {
				return moduleOptions{}, fmt.Errorf("unknown g module flag %q", arg)
			}
			paths = append(paths, arg)
		}
	}
	if len(paths) != 1 {
		return moduleOptions{}, errors.New("g module requires exactly one path")
	}
	options.path = paths[0]
	return options, nil
}

type componentKind string

const (
	componentController componentKind = "controller"
	componentService    componentKind = "service"
	componentListener   componentKind = "listener"
	componentTask       componentKind = "task"
	componentProcessor  componentKind = "processor"
	componentGateway    componentKind = "gateway"
)

type componentOptions struct {
	path         string
	dryRun       bool
	force        bool
	updateModule bool
	test         bool
}

func (c *CLI) runGenerateComponent(ctx context.Context, args []string, kind componentKind) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	options, err := parseComponentOptions(args, kind)
	if err != nil {
		return err
	}
	componentPath, err := parseGeneratorPath(options.path)
	if err != nil {
		return err
	}

	result, err := c.generateComponent(componentPath, kind, options)
	if err != nil {
		return err
	}
	return writeModuleOutput(c.Stdout, result)
}

func parseComponentOptions(args []string, kind componentKind) (componentOptions, error) {
	options := componentOptions{updateModule: true, test: true}
	paths := make([]string, 0, 1)
	command := "g " + string(kind)
	for _, arg := range args {
		switch arg {
		case "--dry-run":
			options.dryRun = true
		case "--force":
			options.force = true
		case "--no-update-module":
			options.updateModule = false
		case "--no-test":
			options.test = false
		default:
			if strings.HasPrefix(arg, "-") {
				return componentOptions{}, fmt.Errorf("unknown %s flag %q", command, arg)
			}
			paths = append(paths, arg)
		}
	}
	if len(paths) != 1 {
		return componentOptions{}, fmt.Errorf("%s requires exactly one path", command)
	}
	options.path = paths[0]
	return options, nil
}

type resourceOptions struct {
	path         string
	dryRun       bool
	force        bool
	updateParent bool
	test         bool
}

func parseResourceOptions(args []string) (resourceOptions, error) {
	options := resourceOptions{updateParent: true, test: true}
	paths := make([]string, 0, 1)
	for _, arg := range args {
		switch arg {
		case "--dry-run":
			options.dryRun = true
		case "--force":
			options.force = true
		case "--no-update-parent":
			options.updateParent = false
		case "--no-test":
			options.test = false
		default:
			if strings.HasPrefix(arg, "-") {
				return resourceOptions{}, fmt.Errorf("unknown g resource flag %q", arg)
			}
			paths = append(paths, arg)
		}
	}
	if len(paths) != 1 {
		return resourceOptions{}, errors.New("g resource requires exactly one path")
	}
	options.path = paths[0]
	return options, nil
}

type moduleGenerateResult struct {
	created       []string
	updated       []string
	warnings      []string
	hints         []string
	dryRun        bool
	noParent      bool
	parentSkipped bool
}

func (c *CLI) generateComponent(componentPath generatorPath, kind componentKind, options componentOptions) (moduleGenerateResult, error) {
	result := moduleGenerateResult{dryRun: options.dryRun, parentSkipped: !options.updateModule}
	target := componentPath.componentFilePath(c.WorkDir, kind)
	content, err := componentFileContent(componentPath, kind)
	if err != nil {
		return result, err
	}

	files := []generatedFileSpec{{path: target, content: content}}
	if options.test {
		testContent, err := componentTestFileContent(componentPath, kind)
		if err != nil {
			return result, err
		}
		files = append(files, generatedFileSpec{
			path:    componentPath.componentTestFilePath(c.WorkDir, kind),
			content: testContent,
		})
	}

	if err := c.writeGeneratedFiles(&result, files, options.force); err != nil {
		return result, err
	}

	if !options.updateModule {
		return result, nil
	}

	module := componentPath.moduleFilePath(c.WorkDir)
	if !fileExists(module) {
		result.noParent = true
		result.warnings = append(result.warnings, "module file not found")
		result.hints = append(result.hints, "add "+providerCall(componentPath, kind)+" manually")
		return result, nil
	}

	relativeModule := slashRel(c.WorkDir, module)
	if options.dryRun {
		result.updated = append(result.updated, relativeModule)
		return result, nil
	}
	updated, err := updateModuleProviders(module, c.WorkDir, componentPath, kind)
	if err != nil {
		return result, err
	}
	if updated {
		result.updated = append(result.updated, relativeModule)
	} else {
		result.warnings = append(result.warnings, "module already provides "+providerCall(componentPath, kind))
	}
	return result, nil
}

type generatedFileSpec struct {
	path    string
	content []byte
}

func (c *CLI) writeGeneratedFiles(result *moduleGenerateResult, files []generatedFileSpec, force bool) error {
	for _, file := range files {
		relative := slashRel(c.WorkDir, file.path)
		if _, err := os.Stat(file.path); err == nil && !force {
			return fmt.Errorf("%s already exists; use --force to overwrite", relative)
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
		result.created = append(result.created, relative)
	}
	if result.dryRun {
		return nil
	}
	for _, file := range files {
		if err := os.MkdirAll(filepath.Dir(file.path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(file.path, file.content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (c *CLI) generateModule(modulePath generatorPath, options moduleOptions) (moduleGenerateResult, error) {
	result := moduleGenerateResult{dryRun: options.dryRun, parentSkipped: !options.updateParent}
	target := modulePath.moduleFilePath(c.WorkDir)
	relativeTarget := slashRel(c.WorkDir, target)
	content, err := moduleFileContent(modulePath)
	if err != nil {
		return result, err
	}

	if _, err := os.Stat(target); err == nil && !options.force {
		return result, fmt.Errorf("%s already exists; use --force to overwrite", relativeTarget)
	} else if err != nil && !os.IsNotExist(err) {
		return result, err
	}

	result.created = append(result.created, relativeTarget)
	if !options.dryRun {
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return result, err
		}
		if err := os.WriteFile(target, content, 0o644); err != nil {
			return result, err
		}
	}

	if !options.updateParent {
		return result, nil
	}

	parent := modulePath.findParentModule(c.WorkDir)
	if parent == "" {
		result.noParent = true
		result.warnings = append(result.warnings, "parent module not found")
		result.hints = append(result.hints, "add "+moduleCall(modulePath)+" manually")
		return result, nil
	}

	relativeParent := slashRel(c.WorkDir, parent)
	if options.dryRun {
		result.updated = append(result.updated, relativeParent)
		return result, nil
	}
	updated, err := updateParentModule(parent, c.WorkDir, modulePath)
	if err != nil {
		return result, err
	}
	if updated {
		result.updated = append(result.updated, relativeParent)
	} else {
		result.warnings = append(result.warnings, "parent module already imports "+moduleCall(modulePath))
	}
	return result, nil
}

func (c *CLI) generateResource(resourcePath generatorPath, options resourceOptions) (moduleGenerateResult, error) {
	result := moduleGenerateResult{dryRun: options.dryRun, parentSkipped: !options.updateParent}
	files, err := resourceFiles(c.WorkDir, resourcePath, options.test)
	if err != nil {
		return result, err
	}
	if err := c.writeGeneratedFiles(&result, files, options.force); err != nil {
		return result, err
	}

	if !options.updateParent {
		return result, nil
	}

	parent := resourcePath.findParentModule(c.WorkDir)
	if parent == "" {
		result.noParent = true
		result.warnings = append(result.warnings, "parent module not found")
		result.hints = append(result.hints, "add "+moduleCall(resourcePath)+" manually")
		return result, nil
	}

	relativeParent := slashRel(c.WorkDir, parent)
	if options.dryRun {
		result.updated = append(result.updated, relativeParent)
		return result, nil
	}
	updated, err := updateParentModule(parent, c.WorkDir, resourcePath)
	if err != nil {
		return result, err
	}
	if updated {
		result.updated = append(result.updated, relativeParent)
	} else {
		result.warnings = append(result.warnings, "parent module already imports "+moduleCall(resourcePath))
	}
	return result, nil
}

type generatorPath struct {
	parts       []string
	slash       string
	packageName string
	typePrefix  string
	moduleName  string
}

func parseGeneratorPath(raw string) (generatorPath, error) {
	raw = strings.Trim(raw, "/")
	if raw == "" || strings.Contains(raw, "..") {
		return generatorPath{}, fmt.Errorf("invalid module path %q", raw)
	}
	parts := strings.Split(raw, "/")
	for _, part := range parts {
		if !isIdentifier(part) {
			return generatorPath{}, fmt.Errorf("invalid module path segment %q", part)
		}
	}
	packageName := parts[len(parts)-1]
	return generatorPath{
		parts:       append([]string(nil), parts...),
		slash:       strings.Join(parts, "/"),
		packageName: packageName,
		typePrefix:  exportedName(packageName),
		moduleName:  strings.Join(parts, "."),
	}, nil
}

func isIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for index, r := range value {
		if index == 0 && !unicode.IsLetter(r) && r != '_' {
			return false
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return true
}

func (p generatorPath) moduleFilePath(workDir string) string {
	return filepath.Join(workDir, "internal", filepath.Join(p.parts...), p.packageName+".module.go")
}

func moduleFileContent(modulePath generatorPath) ([]byte, error) {
	source := fmt.Sprintf(`package %s

import "github.com/r6m/gest"

type Options struct{}

func Module(options Options) gest.Module {
	return gest.NewModule(gest.ModuleConfig{
		Name: %q,
	})
}
`, modulePath.packageName, modulePath.moduleName)
	return format.Source([]byte(source))
}

func (p generatorPath) componentFilePath(workDir string, kind componentKind) string {
	return filepath.Join(workDir, "internal", filepath.Join(p.parts...), p.packageName+"."+string(kind)+".go")
}

func (p generatorPath) componentTestFilePath(workDir string, kind componentKind) string {
	return filepath.Join(workDir, "internal", filepath.Join(p.parts...), p.packageName+"."+string(kind)+"_test.go")
}

func (p generatorPath) dtoFilePath(workDir string) string {
	return filepath.Join(workDir, "internal", filepath.Join(p.parts...), p.packageName+".dto.go")
}

func componentFileContent(componentPath generatorPath, kind componentKind) ([]byte, error) {
	var source string
	switch kind {
	case componentController:
		source = fmt.Sprintf(`package %s

// @Controller("/%s")
type %sController struct{}

func New%sController() *%sController {
	return &%sController{}
}
`, componentPath.packageName, routePath(componentPath), componentPath.typePrefix, componentPath.typePrefix, componentPath.typePrefix, componentPath.typePrefix)
	case componentService:
		source = fmt.Sprintf(`package %s

type %sService struct{}

func New%sService() *%sService {
	return &%sService{}
}
`, componentPath.packageName, componentPath.typePrefix, componentPath.typePrefix, componentPath.typePrefix, componentPath.typePrefix)
	case componentListener:
		source = fmt.Sprintf(`package %s

import (
	"context"

	"github.com/r6m/gest/modules/events"
)

type %sEvent struct {
	ID string
}

// @OnEvent("%s.created")
type %sListener struct {
	bus *events.Bus
}

func New%sListener(bus *events.Bus) *%sListener {
	return &%sListener{bus: bus}
}

func (l *%sListener) OnModuleInit(ctx context.Context) error {
	_ = ctx
	return events.RegisterListener(l.bus, l)
}

func (l *%sListener) Handle(ctx context.Context, event %sEvent) error {
	_ = ctx
	_ = event
	return nil
}
`, componentPath.packageName, componentPath.typePrefix, eventNamePrefix(componentPath), componentPath.typePrefix, componentPath.typePrefix, componentPath.typePrefix, componentPath.typePrefix, componentPath.typePrefix, componentPath.typePrefix, componentPath.typePrefix)
	case componentTask:
		source = fmt.Sprintf(`package %s

import (
	"context"

	"github.com/r6m/gest/modules/scheduler"
)

// @Every("1m")
type %sTask struct {
	scheduler *scheduler.Scheduler
}

func New%sTask(scheduler *scheduler.Scheduler) *%sTask {
	return &%sTask{scheduler: scheduler}
}

func (t *%sTask) OnModuleInit(ctx context.Context) error {
	_ = ctx
	return scheduler.RegisterTask(t.scheduler, t)
}

func (t *%sTask) Run(ctx context.Context) error {
	_ = ctx
	return nil
}
`, componentPath.packageName, componentPath.typePrefix, componentPath.typePrefix, componentPath.typePrefix, componentPath.typePrefix, componentPath.typePrefix, componentPath.typePrefix)
	case componentProcessor:
		source = fmt.Sprintf(`package %s

import (
	"context"

	"github.com/r6m/gest/modules/queue"
)

type %sJob struct {
	ID string
}

// @Processor("%s")
type %sProcessor struct {
	queue *queue.Queue
}

func New%sProcessor(queue *queue.Queue) *%sProcessor {
	return &%sProcessor{queue: queue}
}

func (p *%sProcessor) OnModuleInit(ctx context.Context) error {
	_ = ctx
	return queue.RegisterProcessor(p.queue, p)
}

func (p *%sProcessor) Process(ctx context.Context, job %sJob) error {
	_ = ctx
	_ = job
	return nil
}
`, componentPath.packageName, componentPath.typePrefix, queueName(componentPath), componentPath.typePrefix, componentPath.typePrefix, componentPath.typePrefix, componentPath.typePrefix, componentPath.typePrefix, componentPath.typePrefix, componentPath.typePrefix)
	case componentGateway:
		source = fmt.Sprintf(`package %s

import (
	"context"

	"github.com/r6m/gest/modules/websocket"
)

type %sMessage struct {
	Body string `+"`json:\"body\"`"+`
}

// @Gateway("/ws/%s")
type %sGateway struct{}

func New%sGateway() *%sGateway {
	return &%sGateway{}
}

// @Subscribe("%s.message")
func (g *%sGateway) HandleMessage(ctx context.Context, client *websocket.Client, msg %sMessage) error {
	_ = ctx
	_ = client
	_ = msg
	return nil
}
`, componentPath.packageName, componentPath.typePrefix, routePath(componentPath), componentPath.typePrefix, componentPath.typePrefix, componentPath.typePrefix, componentPath.typePrefix, eventNamePrefix(componentPath), componentPath.typePrefix, componentPath.typePrefix)
	default:
		return nil, fmt.Errorf("unknown component kind %q", kind)
	}
	return format.Source([]byte(source))
}

func componentTestFileContent(componentPath generatorPath, kind componentKind) ([]byte, error) {
	var source string
	switch kind {
	case componentController:
		source = fmt.Sprintf(`package %s

import "testing"

func TestNew%sController(t *testing.T) {
	controller := New%sController()
	if controller == nil {
		t.Fatal("controller is nil")
	}
}
`, componentPath.packageName, componentPath.typePrefix, componentPath.typePrefix)
	case componentService:
		source = fmt.Sprintf(`package %s

import "testing"

func TestNew%sService(t *testing.T) {
	service := New%sService()
	if service == nil {
		t.Fatal("service is nil")
	}
}
`, componentPath.packageName, componentPath.typePrefix, componentPath.typePrefix)
	case componentListener:
		source = fmt.Sprintf(`package %s

import (
	"context"
	"testing"

	"github.com/r6m/gest/modules/events"
)

func TestNew%sListener(t *testing.T) {
	bus := events.NewBus()
	listener := New%sListener(bus)
	if listener == nil {
		t.Fatal("listener is nil")
	}
	if err := listener.OnModuleInit(context.Background()); err != nil {
		t.Fatalf("OnModuleInit returned error: %%v", err)
	}
	if err := bus.Emit(context.Background(), "%s.created", %sEvent{ID: "sample"}); err != nil {
		t.Fatalf("Emit returned error: %%v", err)
	}
}
`, componentPath.packageName, componentPath.typePrefix, componentPath.typePrefix, eventNamePrefix(componentPath), componentPath.typePrefix)
	case componentTask:
		source = fmt.Sprintf(`package %s

import (
	"context"
	"testing"

	"github.com/r6m/gest/modules/scheduler"
)

func TestNew%sTask(t *testing.T) {
	s := scheduler.NewScheduler()
	task := New%sTask(s)
	if task == nil {
		t.Fatal("task is nil")
	}
	if err := task.OnModuleInit(context.Background()); err != nil {
		t.Fatalf("OnModuleInit returned error: %%v", err)
	}
}
`, componentPath.packageName, componentPath.typePrefix, componentPath.typePrefix)
	case componentProcessor:
		source = fmt.Sprintf(`package %s

import (
	"context"
	"testing"

	"github.com/r6m/gest/modules/queue"
)

func TestNew%sProcessor(t *testing.T) {
	q := queue.NewQueue(queue.Options{})
	processor := New%sProcessor(q)
	if processor == nil {
		t.Fatal("processor is nil")
	}
	if err := processor.OnModuleInit(context.Background()); err != nil {
		t.Fatalf("OnModuleInit returned error: %%v", err)
	}
}
`, componentPath.packageName, componentPath.typePrefix, componentPath.typePrefix)
	case componentGateway:
		source = fmt.Sprintf(`package %s

import (
	"context"
	"testing"

	"github.com/r6m/gest/modules/websocket"
)

func TestNew%sGateway(t *testing.T) {
	gateway := New%sGateway()
	if gateway == nil {
		t.Fatal("gateway is nil")
	}
	if err := gateway.HandleMessage(context.Background(), &websocket.Client{}, %sMessage{Body: "sample"}); err != nil {
		t.Fatalf("HandleMessage returned error: %%v", err)
	}
}
`, componentPath.packageName, componentPath.typePrefix, componentPath.typePrefix, componentPath.typePrefix)
	default:
		return nil, fmt.Errorf("unknown component kind %q", kind)
	}
	return format.Source([]byte(source))
}

func resourceFiles(workDir string, resourcePath generatorPath, includeTests bool) ([]generatedFileSpec, error) {
	moduleContent, err := resourceModuleFileContent(resourcePath)
	if err != nil {
		return nil, err
	}
	serviceContent, err := resourceServiceFileContent(resourcePath)
	if err != nil {
		return nil, err
	}
	controllerContent, err := resourceControllerFileContent(resourcePath)
	if err != nil {
		return nil, err
	}
	dtoContent, err := resourceDTOFileContent(resourcePath)
	if err != nil {
		return nil, err
	}
	files := []generatedFileSpec{
		{path: resourcePath.moduleFilePath(workDir), content: moduleContent},
		{path: resourcePath.componentFilePath(workDir, componentService), content: serviceContent},
		{path: resourcePath.componentFilePath(workDir, componentController), content: controllerContent},
		{path: resourcePath.dtoFilePath(workDir), content: dtoContent},
	}
	if includeTests {
		serviceTest, err := resourceServiceTestFileContent(resourcePath)
		if err != nil {
			return nil, err
		}
		controllerTest, err := resourceControllerTestFileContent(resourcePath)
		if err != nil {
			return nil, err
		}
		files = append(files,
			generatedFileSpec{path: resourcePath.componentTestFilePath(workDir, componentService), content: serviceTest},
			generatedFileSpec{path: resourcePath.componentTestFilePath(workDir, componentController), content: controllerTest},
		)
	}
	return files, nil
}

func resourceModuleFileContent(resourcePath generatorPath) ([]byte, error) {
	source := fmt.Sprintf(`package %s

import "github.com/r6m/gest"

type Options struct{}

func Module(options Options) gest.Module {
	return gest.NewModule(gest.ModuleConfig{
		Name: %q,
		Providers: gest.Providers(
			gest.Provide(New%sService),
			gest.Controller(New%sController),
		),
	})
}
`, resourcePath.packageName, resourcePath.moduleName, resourcePath.typePrefix, resourcePath.typePrefix)
	return format.Source([]byte(source))
}

func resourceServiceFileContent(resourcePath generatorPath) ([]byte, error) {
	source := fmt.Sprintf(`package %s

type %s struct {
	ID   string
	Name string
}

type %sService struct{}

func New%sService() *%sService {
	return &%sService{}
}

func (s *%sService) List() []%s {
	return []%s{
		{ID: "sample", Name: "Sample %s"},
	}
}
`, resourcePath.packageName, resourcePath.typePrefix, resourcePath.typePrefix, resourcePath.typePrefix, resourcePath.typePrefix, resourcePath.typePrefix, resourcePath.typePrefix, resourcePath.typePrefix, resourcePath.typePrefix, resourcePath.typePrefix)
	return format.Source([]byte(source))
}

func resourceControllerFileContent(resourcePath generatorPath) ([]byte, error) {
	source := fmt.Sprintf(`package %s

import "github.com/r6m/gest"

// @Controller("/%s")
type %sController struct {
	service *%sService
}

func New%sController(service *%sService) *%sController {
	return &%sController{service: service}
}

// @Get("/")
func (c *%sController) List(ctx *gest.Context) (*List%sResponse, error) {
	items := c.service.List()
	response := List%sResponse{
		Items: make([]%sResponse, 0, len(items)),
	}
	for _, item := range items {
		response.Items = append(response.Items, %sResponse{
			ID:   item.ID,
			Name: item.Name,
		})
	}
	return &response, nil
}
`, resourcePath.packageName, routePath(resourcePath), resourcePath.typePrefix, resourcePath.typePrefix, resourcePath.typePrefix, resourcePath.typePrefix, resourcePath.typePrefix, resourcePath.typePrefix, resourcePath.typePrefix, resourcePath.typePrefix, resourcePath.typePrefix, resourcePath.typePrefix, resourcePath.typePrefix)
	return format.Source([]byte(source))
}

func resourceDTOFileContent(resourcePath generatorPath) ([]byte, error) {
	source := fmt.Sprintf(`package %s

type %sResponse struct {
	ID   string `+"`json:\"id\"`"+`
	Name string `+"`json:\"name\"`"+`
}

type List%sResponse struct {
	Items []%sResponse `+"`json:\"items\"`"+`
}
`, resourcePath.packageName, resourcePath.typePrefix, resourcePath.typePrefix, resourcePath.typePrefix)
	return format.Source([]byte(source))
}

func resourceServiceTestFileContent(resourcePath generatorPath) ([]byte, error) {
	source := fmt.Sprintf(`package %s

import "testing"

func Test%sServiceListReturnsSample(t *testing.T) {
	service := New%sService()
	items := service.List()
	if len(items) != 1 {
		t.Fatalf("items length = %%d, want 1", len(items))
	}
	if items[0].ID == "" || items[0].Name == "" {
		t.Fatalf("item = %%#v, want populated sample", items[0])
	}
}
`, resourcePath.packageName, resourcePath.typePrefix, resourcePath.typePrefix)
	return format.Source([]byte(source))
}

func resourceControllerTestFileContent(resourcePath generatorPath) ([]byte, error) {
	source := fmt.Sprintf(`package %s

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/r6m/gest"
)

func Test%sControllerList(t *testing.T) {
	controller := New%sController(New%sService())
	ctx := gest.NewContext(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	response, err := controller.List(ctx)
	if err != nil {
		t.Fatalf("List returned error: %%v", err)
	}
	if response == nil || len(response.Items) != 1 {
		t.Fatalf("response = %%#v, want one item", response)
	}
}
`, resourcePath.packageName, resourcePath.typePrefix, resourcePath.typePrefix, resourcePath.typePrefix)
	return format.Source([]byte(source))
}

func exportedName(value string) string {
	parts := strings.Split(value, "_")
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, "")
}

func routePath(componentPath generatorPath) string {
	return componentPath.packageName
}

func (p generatorPath) findParentModule(workDir string) string {
	for depth := len(p.parts) - 1; depth > 0; depth-- {
		parentParts := p.parts[:depth]
		parent := filepath.Join(append([]string{workDir, "internal"}, parentParts...)...)
		path := filepath.Join(parent, parentParts[len(parentParts)-1]+".module.go")
		if fileExists(path) {
			return path
		}
	}
	rootApp := filepath.Join(workDir, "internal", "app", "app.module.go")
	if fileExists(rootApp) {
		return rootApp
	}
	return ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func updateParentModule(path string, workDir string, modulePath generatorPath) (bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	moduleImport, err := importPathFor(workDir, modulePath)
	if err != nil {
		return false, err
	}

	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, path, content, parser.ParseComments)
	if err != nil {
		return false, fmt.Errorf("parse parent module %s: %w", slashRel(workDir, path), err)
	}
	if parentHasModuleCall(parsed, modulePath) {
		return false, nil
	}

	edits := make([]textEdit, 0, 2)
	if !hasImport(parsed, moduleImport) {
		edit, err := importEdit(fileSet, parsed, moduleImport)
		if err != nil {
			return false, err
		}
		edits = append(edits, edit)
	}
	edit, err := importsEdit(fileSet, parsed, modulePath)
	if err != nil {
		return false, err
	}
	edits = append(edits, edit)

	updated := applyEdits(content, edits)
	formatted, err := format.Source(updated)
	if err != nil {
		return false, fmt.Errorf("format parent module %s: %w", slashRel(workDir, path), err)
	}
	if bytes.Equal(content, formatted) {
		return false, nil
	}
	if err := os.WriteFile(path, formatted, 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func updateModuleProviders(path string, workDir string, componentPath generatorPath, kind componentKind) (bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, path, content, parser.ParseComments)
	if err != nil {
		return false, fmt.Errorf("parse module %s: %w", slashRel(workDir, path), err)
	}
	call := providerCall(componentPath, kind)
	if fileHasCall(parsed, call) {
		return false, nil
	}

	edits := []textEdit{}
	edit, err := providersEdit(fileSet, parsed, call)
	if err != nil {
		return false, err
	}
	edits = append(edits, edit)
	if kind == componentGateway && !hasImport(parsed, "github.com/r6m/gest/modules/websocket") {
		edit, err := importEdit(fileSet, parsed, "github.com/r6m/gest/modules/websocket")
		if err != nil {
			return false, err
		}
		edits = append(edits, edit)
	}
	updated := applyEdits(content, edits)
	formatted, err := format.Source(updated)
	if err != nil {
		return false, fmt.Errorf("format module %s: %w", slashRel(workDir, path), err)
	}
	if bytes.Equal(content, formatted) {
		return false, nil
	}
	if err := os.WriteFile(path, formatted, 0o644); err != nil {
		return false, err
	}
	return true, nil
}

type textEdit struct {
	offset int
	text   string
}

func applyEdits(content []byte, edits []textEdit) []byte {
	slices.SortFunc(edits, func(a textEdit, b textEdit) int {
		return b.offset - a.offset
	})
	updated := append([]byte(nil), content...)
	for _, edit := range edits {
		updated = append(updated[:edit.offset], append([]byte(edit.text), updated[edit.offset:]...)...)
	}
	return updated
}

func hasImport(file *ast.File, importPath string) bool {
	quoted := strconvQuote(importPath)
	for _, spec := range file.Imports {
		if spec.Path != nil && spec.Path.Value == quoted {
			return true
		}
	}
	return false
}

func importEdit(fileSet *token.FileSet, file *ast.File, importPath string) (textEdit, error) {
	quoted := strconvQuote(importPath)
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.IMPORT {
			continue
		}
		if general.Lparen.IsValid() {
			return textEdit{
				offset: fileSet.Position(general.Rparen).Offset,
				text:   "\t" + quoted + "\n",
			}, nil
		}
		return textEdit{
			offset: fileSet.Position(general.End()).Offset,
			text:   "\n\nimport " + quoted,
		}, nil
	}
	return textEdit{
		offset: fileSet.Position(file.Name.End()).Offset,
		text:   "\n\nimport " + quoted,
	}, nil
}

func importsEdit(fileSet *token.FileSet, file *ast.File, modulePath generatorPath) (textEdit, error) {
	call := moduleCall(modulePath)
	var moduleConfig *ast.CompositeLit
	ast.Inspect(file, func(node ast.Node) bool {
		if moduleConfig != nil {
			return false
		}
		lit, ok := node.(*ast.CompositeLit)
		if !ok || !isGestModuleConfig(lit.Type) {
			return true
		}
		moduleConfig = lit
		return false
	})
	if moduleConfig == nil {
		return textEdit{}, errors.New("parent module does not contain gest.ModuleConfig")
	}

	for _, element := range moduleConfig.Elts {
		kv, ok := element.(*ast.KeyValueExpr)
		if !ok || identName(kv.Key) != "Imports" {
			continue
		}
		callExpr, ok := kv.Value.(*ast.CallExpr)
		if !ok || !isGestImports(callExpr.Fun) {
			return textEdit{}, errors.New("parent module Imports field is not gest.Imports(...)")
		}
		if len(callExpr.Args) > 0 {
			offset := fileSet.Position(callExpr.Args[len(callExpr.Args)-1].End()).Offset
			return textEdit{offset: offset, text: ",\n\t\t\t" + call}, nil
		}
		offset := fileSet.Position(callExpr.Rparen).Offset
		return textEdit{offset: offset, text: "\t\t\t" + call + ","}, nil
	}

	offset := fileSet.Position(moduleConfig.Rbrace).Offset
	return textEdit{
		offset: offset,
		text:   "\t\tImports: gest.Imports(\n\t\t\t" + call + ",\n\t\t),\n",
	}, nil
}

func providersEdit(fileSet *token.FileSet, file *ast.File, call string) (textEdit, error) {
	moduleConfig := findModuleConfig(file)
	if moduleConfig == nil {
		return textEdit{}, errors.New("module file does not contain gest.ModuleConfig")
	}

	for _, element := range moduleConfig.Elts {
		kv, ok := element.(*ast.KeyValueExpr)
		if !ok || identName(kv.Key) != "Providers" {
			continue
		}
		callExpr, ok := kv.Value.(*ast.CallExpr)
		if !ok || !isGestProviders(callExpr.Fun) {
			return textEdit{}, errors.New("module Providers field is not gest.Providers(...)")
		}
		if len(callExpr.Args) > 0 {
			offset := fileSet.Position(callExpr.Args[len(callExpr.Args)-1].End()).Offset
			return textEdit{offset: offset, text: ",\n\t\t\t" + call}, nil
		}
		offset := fileSet.Position(callExpr.Rparen).Offset
		return textEdit{offset: offset, text: "\t\t\t" + call + ","}, nil
	}

	offset := fileSet.Position(moduleConfig.Rbrace).Offset
	return textEdit{
		offset: offset,
		text:   "\t\tProviders: gest.Providers(\n\t\t\t" + call + ",\n\t\t),\n",
	}, nil
}

func findModuleConfig(file *ast.File) *ast.CompositeLit {
	var moduleConfig *ast.CompositeLit
	ast.Inspect(file, func(node ast.Node) bool {
		if moduleConfig != nil {
			return false
		}
		lit, ok := node.(*ast.CompositeLit)
		if !ok || !isGestModuleConfig(lit.Type) {
			return true
		}
		moduleConfig = lit
		return false
	})
	return moduleConfig
}

func isGestModuleConfig(expr ast.Expr) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	return ok && selector.Sel != nil && selector.Sel.Name == "ModuleConfig" && identName(selector.X) == "gest"
}

func isGestImports(expr ast.Expr) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	return ok && selector.Sel != nil && selector.Sel.Name == "Imports" && identName(selector.X) == "gest"
}

func isGestProviders(expr ast.Expr) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	return ok && selector.Sel != nil && selector.Sel.Name == "Providers" && identName(selector.X) == "gest"
}

func identName(expr ast.Expr) string {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return ""
	}
	return ident.Name
}

func parentHasModuleCall(file *ast.File, modulePath generatorPath) bool {
	return fileHasCall(file, moduleCall(modulePath))
}

func fileHasCall(file *ast.File, call string) bool {
	found := false
	ast.Inspect(file, func(node ast.Node) bool {
		if found {
			return false
		}
		expr, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		found = callExprString(expr) == call
		return !found
	})
	return found
}

func callExprString(expr ast.Expr) string {
	var buffer bytes.Buffer
	if err := format.Node(&buffer, token.NewFileSet(), expr); err != nil {
		return ""
	}
	return buffer.String()
}

func moduleCall(modulePath generatorPath) string {
	return modulePath.packageName + ".Module(" + modulePath.packageName + ".Options{})"
}

func providerCall(componentPath generatorPath, kind componentKind) string {
	switch kind {
	case componentController:
		return "gest.Controller(New" + componentPath.typePrefix + "Controller)"
	case componentService:
		return "gest.Provide(New" + componentPath.typePrefix + "Service)"
	case componentListener:
		return "gest.Provide(New" + componentPath.typePrefix + "Listener)"
	case componentTask:
		return "gest.Provide(New" + componentPath.typePrefix + "Task)"
	case componentProcessor:
		return "gest.Provide(New" + componentPath.typePrefix + "Processor)"
	case componentGateway:
		return "websocket.Gateway(New" + componentPath.typePrefix + "Gateway)"
	default:
		return ""
	}
}

func eventNamePrefix(componentPath generatorPath) string {
	return strings.ReplaceAll(componentPath.moduleName, "_", ".")
}

func queueName(componentPath generatorPath) string {
	return strings.ReplaceAll(componentPath.moduleName, "_", ".")
}

func importPathFor(workDir string, modulePath generatorPath) (string, error) {
	moduleName, err := readGoModulePath(filepath.Join(workDir, "go.mod"))
	if err != nil {
		return "", err
	}
	return moduleName + "/internal/" + modulePath.slash, nil
}

func readGoModulePath(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	return "", fmt.Errorf("%s does not declare a module path", path)
}

func slashRel(root string, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}

func strconvQuote(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

func writeModuleOutput(w io.Writer, result moduleGenerateResult) error {
	if w == nil {
		w = io.Discard
	}
	prefix := ""
	if result.dryRun {
		prefix = "DRY-RUN "
	}
	for _, path := range result.created {
		if _, err := fmt.Fprintf(w, "%sCREATE %s\n", prefix, path); err != nil {
			return err
		}
	}
	for _, path := range result.updated {
		if _, err := fmt.Fprintf(w, "%sUPDATE %s\n", prefix, path); err != nil {
			return err
		}
	}
	if result.parentSkipped {
		if _, err := fmt.Fprintln(w, "SKIP parent module update"); err != nil {
			return err
		}
	}
	for _, warning := range result.warnings {
		if _, err := fmt.Fprintf(w, "WARN %s\n", warning); err != nil {
			return err
		}
	}
	for _, hint := range result.hints {
		if _, err := fmt.Fprintf(w, "HINT %s\n", hint); err != nil {
			return err
		}
	}
	return nil
}
