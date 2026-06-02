package gest

import "fmt"

// Guard decides whether a request may continue to the route handler.
type Guard interface {
	CanActivate(ctx *Context) error
}

// GuardFunc adapts a function into a Guard.
type GuardFunc func(ctx *Context) error

// CanActivate runs the guard function.
func (g GuardFunc) CanActivate(ctx *Context) error {
	return g(ctx)
}

// GuardFactory resolves a route guard from the application container.
type GuardFactory func(container Container) (Guard, error)

// ResolveGuard resolves a DI provider and requires it to implement Guard.
func ResolveGuard[T any]() GuardFactory {
	return func(container Container) (Guard, error) {
		value, err := container.Resolve(TokenOf[T]())
		if err != nil {
			return nil, err
		}
		guard, ok := value.(Guard)
		if !ok {
			return nil, fmt.Errorf("resolved guard %s does not implement gest.Guard", TokenOf[T]())
		}
		return guard, nil
	}
}

// GuardedHandler returns a handler that runs guards before the route handler.
func GuardedHandler(handler HandlerFunc, guards []Guard) HandlerFunc {
	if len(guards) == 0 {
		return handler
	}

	return func(ctx *Context) error {
		for _, guard := range guards {
			if err := guard.CanActivate(ctx); err != nil {
				return err
			}
		}
		return handler(ctx)
	}
}
