package gest

// RouteComponentFactory resolves a @Use(...) route component from the application container.
type RouteComponentFactory func(container Container) (any, error)

// ResolveRouteComponent resolves a DI provider for unified @Use(...) classification.
func ResolveRouteComponent[T any]() RouteComponentFactory {
	return func(container Container) (any, error) {
		return container.Resolve(TokenOf[T]())
	}
}
