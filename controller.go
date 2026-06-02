package gest

// DescribedController exposes explicit runtime controller metadata.
type DescribedController interface {
	GestController() ControllerDefinition
}

// ControllerDefinition describes a controller and its routes.
type ControllerDefinition struct {
	Name        string
	BasePath    string
	Tag         string
	Components  []RouteComponentFactory
	Middlewares []MiddlewareFactory
	Guards      []GuardFactory
	Routes      []RouteDefinition
}

// RouteDefinition describes a route and its runtime handler.
type RouteDefinition struct {
	Name        string
	Method      string
	Path        string
	Handler     HandlerFunc
	Request     any
	Response    any
	Statuses    []int
	Metadata    RouteMetadata
	Components  []RouteComponentFactory
	Guards      []GuardFactory
	Middlewares []MiddlewareFactory
}

// RouteMetadata carries optional route metadata used by generators and runtime adapters.
type RouteMetadata struct {
	Summary     string
	Description string
	Auth        bool
	Public      bool
	Roles       []string
	Permissions []string
}

// HandlerFunc is the runtime route handler shape.
type HandlerFunc func(ctx *Context) error
