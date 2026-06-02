package gest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// Option configures an App.
type Option func(*App)

// App wires modules, DI, controllers, and a router adapter.
type App struct {
	router        RouterAdapter
	modules       []Module
	validator     Validator
	routes        []OpenAPIRoute
	openapi       *openAPIConfig
	bootLogs      bool
	bootLogWriter io.Writer
	lifecycle     []*providerState
	built         bool
	shutdown      bool
}

// New creates an application with default options.
func New(options ...Option) *App {
	app := &App{
		router: newDefaultRouter(),
	}
	for _, option := range options {
		option(app)
	}
	return app
}

// WithRouter configures the router adapter.
func WithRouter(adapter RouterAdapter) Option {
	return func(app *App) {
		if adapter != nil {
			app.router = adapter
		}
	}
}

// WithBootLogs enables or disables human-readable boot logs.
func WithBootLogs(enabled bool) Option {
	return func(app *App) {
		app.bootLogs = enabled
	}
}

// WithBootLogWriter configures the writer used for boot logs.
func WithBootLogWriter(writer io.Writer) Option {
	return func(app *App) {
		app.bootLogWriter = writer
	}
}

// WithLogger configures the writer used for boot logs.
func WithLogger(writer io.Writer) Option {
	return WithBootLogWriter(writer)
}

// WithValidator configures the validator used by typed JSON handlers.
func WithValidator(validator Validator) Option {
	return func(app *App) {
		app.validator = validator
	}
}

// Import adds modules to the application.
func (a *App) Import(modules ...Module) {
	a.modules = append(a.modules, modules...)
}

// OpenAPIRoutes returns the route metadata collected during bootstrap.
func (a *App) OpenAPIRoutes() []OpenAPIRoute {
	return cloneOpenAPIRoutes(a.routes)
}

// OpenAPI serves an OpenAPI JSON document built from registered route metadata.
func (a *App) OpenAPI(routePath string, options ...OpenAPIOption) {
	config := newOpenAPIConfig(routePath, options...)
	a.openapi = &config
}

// Listen builds the app and starts the configured router.
func (a *App) Listen(addr string) error {
	if err := a.bootstrap(); err != nil {
		return err
	}
	a.logBoot("GEST listen address: %s", addr)
	return a.router.Serve(addr)
}

// ServeHTTP builds the app if needed and serves a request through the configured router.
func (a *App) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if err := a.bootstrap(); err != nil {
		_ = WriteError(response, err)
		return
	}
	handler, ok := a.router.(http.Handler)
	if !ok {
		_ = WriteError(response, Internal("router does not support in-memory HTTP serving"))
		return
	}
	handler.ServeHTTP(response, request)
}

// Shutdown runs application shutdown lifecycle hooks for initialized providers.
func (a *App) Shutdown(ctx context.Context) error {
	if a.shutdown {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if err := a.callShutdownHook(ctx, "BeforeApplicationShutdown"); err != nil {
		return err
	}
	if err := a.callShutdownHook(ctx, "OnModuleDestroy"); err != nil {
		return err
	}
	if err := a.callShutdownHook(ctx, "OnApplicationShutdown"); err != nil {
		return err
	}

	a.shutdown = true
	return nil
}

func (a *App) bootstrap() error {
	if a.built {
		return nil
	}

	start := time.Now()
	a.logBoot("GEST starting application")

	root := NewModule(ModuleConfig{
		Name:    "App",
		Imports: Imports(a.modules...),
	})
	builder := containerBuilder{}
	rootContainer, err := builder.build(root)
	if err != nil {
		return err
	}

	controllers, err := a.resolveControllers(rootContainer)
	if err != nil {
		return err
	}
	a.lifecycle = initializedProviders(rootContainer)

	if err := a.callStartupHook(context.Background(), "OnModuleInit"); err != nil {
		return err
	}

	seenRoutes := make(map[string]struct{})
	for _, controller := range controllers {
		if err := a.registerController(controller, seenRoutes); err != nil {
			return err
		}
	}
	if err := a.registerOpenAPI(seenRoutes); err != nil {
		return err
	}
	if err := a.callStartupHook(context.Background(), "OnApplicationBootstrap"); err != nil {
		return err
	}

	a.built = true
	a.logBoot("GEST boot duration: %s", time.Since(start).Round(time.Millisecond))
	return nil
}

type controllerRegistration struct {
	definition ControllerDefinition
}

func (a *App) resolveControllers(rootContainer *moduleContainer) ([]controllerRegistration, error) {
	controllers := []controllerRegistration{}
	seenProviders := make(map[*providerState]struct{})
	for _, module := range allModuleContainers(rootContainer) {
		a.logModuleBoot(module)
		for _, provider := range module.ownOrder {
			if provider.provider.Kind != ProviderKindController {
				continue
			}
			if _, ok := seenProviders[provider]; ok {
				continue
			}
			seenProviders[provider] = struct{}{}
			registration, err := resolveController(provider)
			if err != nil {
				return nil, err
			}
			controllers = append(controllers, registration)
		}
	}
	return controllers, nil
}

func (a *App) registerOpenAPI(seenRoutes map[string]struct{}) error {
	if a.openapi == nil {
		return nil
	}
	fullPath := joinRoutePath("", a.openapi.Path)
	key := http.MethodGet + " " + fullPath
	if _, ok := seenRoutes[key]; ok {
		return duplicateRouteError(key)
	}
	seenRoutes[key] = struct{}{}

	document, err := buildOpenAPIDocument(*a.openapi, a.routes)
	if err != nil {
		return err
	}
	a.router.Handle(RouteRuntimeConfig{
		Method: http.MethodGet,
		Path:   fullPath,
		Handler: func(ctx *Context) error {
			return writeOpenAPIDocument(ctx, document)
		},
		Validator: a.validator,
	})
	a.logBoot("GEST OpenAPI route: GET %s", fullPath)
	return nil
}

func resolveController(provider *providerState) (controllerRegistration, error) {
	value, err := provider.resolve(nil)
	if err != nil {
		return controllerRegistration{}, err
	}

	controller, ok := value.(DescribedController)
	if !ok {
		return controllerRegistration{}, controllerMetadataError(provider)
	}

	return controllerRegistration{
		definition: controller.GestController(),
	}, nil
}

func (a *App) registerController(controller controllerRegistration, seenRoutes map[string]struct{}) error {
	definition := controller.definition
	for _, route := range definition.Routes {
		fullPath := joinRoutePath(definition.BasePath, route.Path)
		key := strings.ToUpper(route.Method) + " " + fullPath
		if _, ok := seenRoutes[key]; ok {
			return duplicateRouteError(key)
		}
		seenRoutes[key] = struct{}{}
		a.routes = append(a.routes, newOpenAPIRoute(definition, route, fullPath))
		a.router.Handle(RouteRuntimeConfig{
			Method:    route.Method,
			Path:      fullPath,
			Handler:   route.Handler,
			Validator: a.validator,
		})
		a.logBoot("GEST route: %s %s -> %s.%s", strings.ToUpper(route.Method), fullPath, definition.Name, route.Name)
	}

	return nil
}

func (a *App) callStartupHook(ctx context.Context, hook string) error {
	for _, provider := range a.lifecycle {
		if err := callLifecycleHook(ctx, provider, hook); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) callShutdownHook(ctx context.Context, hook string) error {
	for i := len(a.lifecycle) - 1; i >= 0; i-- {
		if err := callLifecycleHook(ctx, a.lifecycle[i], hook); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) logModuleBoot(module *moduleContainer) {
	providers := 0
	controllers := 0
	for _, provider := range module.ownOrder {
		switch provider.provider.Kind {
		case ProviderKindController:
			controllers++
		default:
			providers++
		}
	}
	a.logBoot("GEST module: %s eager providers=%d controllers=%d", module.name, providers, controllers)
}

func (a *App) logBoot(format string, args ...any) {
	if !a.bootLogs {
		return
	}
	writer := a.bootLogWriter
	if writer == nil {
		writer = os.Stdout
	}
	_, _ = fmt.Fprintf(writer, format+"\n", args...)
}

func allModuleContainers(root *moduleContainer) []*moduleContainer {
	modules := []*moduleContainer{root}
	for _, imported := range root.imports {
		modules = append(modules, allModuleContainers(imported)...)
	}
	return modules
}

func initializedProviders(root *moduleContainer) []*providerState {
	providers := []*providerState{}
	seen := make(map[*providerState]struct{})
	for _, module := range allModuleContainers(root) {
		for _, provider := range module.ownOrder {
			if !provider.initialized {
				continue
			}
			if _, ok := seen[provider]; ok {
				continue
			}
			seen[provider] = struct{}{}
			providers = append(providers, provider)
		}
	}
	return providers
}

func joinRoutePath(basePath string, routePath string) string {
	if basePath == "" {
		basePath = "/"
	}
	if routePath == "" {
		routePath = "/"
	}
	joined := path.Join("/", basePath, routePath)
	if strings.HasSuffix(routePath, "/") && joined != "/" {
		joined += "/"
	}
	return joined
}

type appError struct {
	Code    string
	Message string
	Hint    string
}

func (e *appError) Error() string {
	if e.Hint == "" {
		return e.Code + ": " + e.Message
	}
	return e.Code + ": " + e.Message + ". Hint: " + e.Hint
}

func controllerMetadataError(provider *providerState) error {
	return &appError{
		Code:    "ROUTE_MISSING_CONTROLLER_METADATA",
		Message: "controller " + describeProvider(provider.provider) + " does not implement DescribedController",
		Hint:    "add hand-written GestController() metadata or run the generator",
	}
}

func duplicateRouteError(route string) error {
	return &appError{
		Code:    "ROUTE_DUPLICATE",
		Message: "duplicate route " + route,
		Hint:    "remove one route or change its method/path",
	}
}

type defaultRouter struct {
	router chi.Router
}

func newDefaultRouter() *defaultRouter {
	return &defaultRouter{router: chi.NewRouter()}
}

func (r *defaultRouter) Name() string {
	return "chi"
}

func (r *defaultRouter) Group(prefix string, fn func(group RouterAdapter)) {
	r.router.Route(prefix, func(router chi.Router) {
		fn(&defaultRouter{router: router})
	})
}

func (r *defaultRouter) Handle(route RouteRuntimeConfig) {
	r.router.MethodFunc(strings.ToUpper(route.Method), route.Path, func(response http.ResponseWriter, request *http.Request) {
		context := NewContext(response, request)
		context.SetValidator(route.Validator)
		for _, key := range chi.RouteContext(request.Context()).URLParams.Keys {
			context.SetParam(key, chi.URLParam(request, key))
		}
		if err := route.Handler(context); err != nil {
			_ = WriteError(response, err)
		}
	})
}

func (r *defaultRouter) Use(middleware Middleware) {
	r.router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			handler := func(context *Context) error {
				next.ServeHTTP(context.RawResponse(), context.RawRequest())
				return nil
			}
			context := NewContext(response, request)
			for _, key := range chi.RouteContext(request.Context()).URLParams.Keys {
				context.SetParam(key, chi.URLParam(request, key))
			}
			if err := middleware(handler)(context); err != nil {
				_ = WriteError(response, err)
			}
		})
	})
}

func (r *defaultRouter) Serve(addr string) error {
	return http.ListenAndServe(addr, r.router)
}

func (r *defaultRouter) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	r.router.ServeHTTP(response, request)
}
