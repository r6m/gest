package gest

import (
	"net/http"
	"path"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Option configures an App.
type Option func(*App)

// App wires modules, DI, controllers, and a router adapter.
type App struct {
	router    RouterAdapter
	modules   []Module
	validator Validator
	routes    []OpenAPIRoute
	bootLogs  bool
	built     bool
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

// WithBootLogs stores the boot log setting. Boot logging is not implemented yet.
func WithBootLogs(enabled bool) Option {
	return func(app *App) {
		app.bootLogs = enabled
	}
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

// Listen builds the app and starts the configured router.
func (a *App) Listen(addr string) error {
	if err := a.bootstrap(); err != nil {
		return err
	}
	return a.router.Serve(addr)
}

func (a *App) bootstrap() error {
	if a.built {
		return nil
	}

	root := NewModule(ModuleConfig{
		Name:    "App",
		Imports: Imports(a.modules...),
	})
	builder := containerBuilder{}
	rootContainer, err := builder.build(root)
	if err != nil {
		return err
	}

	seenRoutes := make(map[string]struct{})
	seenProviders := make(map[*providerState]struct{})
	for _, module := range allModuleContainers(rootContainer) {
		for _, provider := range module.own {
			if provider.provider.Kind != ProviderKindController {
				continue
			}
			if _, ok := seenProviders[provider]; ok {
				continue
			}
			seenProviders[provider] = struct{}{}
			if err := a.registerController(provider, seenRoutes); err != nil {
				return err
			}
		}
	}

	a.built = true
	return nil
}

func (a *App) registerController(provider *providerState, seenRoutes map[string]struct{}) error {
	value, err := provider.resolve(nil)
	if err != nil {
		return err
	}

	controller, ok := value.(DescribedController)
	if !ok {
		return controllerMetadataError(provider)
	}

	definition := controller.GestController()
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
	}

	return nil
}

func allModuleContainers(root *moduleContainer) []*moduleContainer {
	modules := []*moduleContainer{root}
	for _, imported := range root.imports {
		modules = append(modules, allModuleContainers(imported)...)
	}
	return modules
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
