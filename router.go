package gest

// RouterAdapter registers runtime routes and serves HTTP traffic.
type RouterAdapter interface {
	Name() string
	Group(prefix string, fn func(group RouterAdapter))
	Handle(route RouteRuntimeConfig)
	Use(middleware Middleware)
	Serve(addr string) error
}

// RouteRuntimeConfig is the explicit runtime route registration input.
//
// Duplicate and invalid route validation belongs to app bootstrap. Router
// adapters register this plain data and may still surface router-native errors
// or panics for invalid configuration until bootstrap validation is implemented.
type RouteRuntimeConfig struct {
	Method     string
	Path       string
	Handler    HandlerFunc
	Middleware []Middleware
	Guards     []Guard
	Validator  Validator
}

// RouteRegistrationContext lets optional modules register HTTP routes after DI is built.
type RouteRegistrationContext struct {
	Container Container
	Register  func(RouteRuntimeConfig) error
}

// RouteRegistrar is implemented by optional module providers that register routes.
type RouteRegistrar interface {
	RegisterRoutes(ctx RouteRegistrationContext) error
}
