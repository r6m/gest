package gest

// DescribedController exposes explicit runtime controller metadata.
type DescribedController interface {
	GestController() ControllerDefinition
}

// ControllerDefinition describes a controller and its routes.
type ControllerDefinition struct {
	Name     string
	BasePath string
	Tag      string
	Routes   []RouteDefinition
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
	Guards      []GuardFactory
	Middlewares []Middleware
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

// GuardFactory is a placeholder for later guard resolution behavior.
type GuardFactory func(Container) (any, error)

// Middleware is a placeholder for later middleware adapter behavior.
type Middleware func(HandlerFunc) HandlerFunc
