package gest

import "fmt"

// Middleware wraps a route handler with request/response behavior.
type Middleware interface {
	Handle(next HandlerFunc) HandlerFunc
}

// MiddlewareFunc adapts a function into Middleware.
type MiddlewareFunc func(next HandlerFunc) HandlerFunc

// Handle wraps next with the middleware function.
func (f MiddlewareFunc) Handle(next HandlerFunc) HandlerFunc {
	return f(next)
}

// MiddlewareHandler returns a handler wrapped by middleware in declaration order.
func MiddlewareHandler(handler HandlerFunc, middlewares []Middleware) HandlerFunc {
	for index := len(middlewares) - 1; index >= 0; index-- {
		handler = middlewares[index].Handle(handler)
	}
	return handler
}

// MiddlewareFactory resolves route middleware from the application container.
type MiddlewareFactory func(container Container) (Middleware, error)

// ResolveMiddleware resolves a DI provider and requires it to implement Middleware.
func ResolveMiddleware[T any]() MiddlewareFactory {
	return func(container Container) (Middleware, error) {
		value, err := container.Resolve(TokenOf[T]())
		if err != nil {
			return nil, err
		}
		middleware, ok := value.(Middleware)
		if !ok {
			return nil, fmt.Errorf("resolved middleware %s does not implement gest.Middleware", TokenOf[T]())
		}
		return middleware, nil
	}
}
