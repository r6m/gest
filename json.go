package gest

import (
	"fmt"
	"reflect"
)

var (
	contextPointerType = reflect.TypeOf((*Context)(nil))
	errorType          = reflect.TypeOf((*error)(nil)).Elem()
)

// Handle wraps a typed controller handler as a runtime HandlerFunc.
func Handle(handler any, options ...HandlerOption) HandlerFunc {
	config := newHandlerConfig(options...)
	handlerValue := reflect.ValueOf(handler)
	if !handlerValue.IsValid() {
		return func(ctx *Context) error {
			return Internal("unsupported handler signature <nil>")
		}
	}
	if handlerValue.Kind() == reflect.Func && handlerValue.IsNil() {
		return func(ctx *Context) error {
			return Internal("unsupported handler signature <nil>")
		}
	}
	handlerType := handlerValue.Type()

	switch {
	case isContextResponseErrorHandler(handlerType):
		return func(ctx *Context) error {
			return callContextResponseErrorHandler(handlerValue, ctx, config)
		}
	case isContextErrorHandler(handlerType):
		return func(ctx *Context) error {
			return callContextErrorHandler(handlerValue, ctx, config)
		}
	case isRequestErrorHandler(handlerType):
		return func(ctx *Context) error {
			return callRequestErrorHandler(handlerValue, ctx, config)
		}
	case isRequestResponseErrorHandler(handlerType):
		return func(ctx *Context) error {
			return callRequestResponseErrorHandler(handlerValue, ctx, config)
		}
	default:
		return func(ctx *Context) error {
			return Internal(fmt.Sprintf("unsupported handler signature %s", handlerType))
		}
	}
}

func HandleContext(handler func(ctx *Context) error, options ...HandlerOption) HandlerFunc {
	config := newHandlerConfig(options...)

	return func(ctx *Context) error {
		if err := handler(ctx); err != nil {
			return err
		}

		return ctx.NoContent(config.emptyStatus)
	}
}

func HandleRequest[Req any](handler func(ctx *Context, req *Req) error, options ...HandlerOption) HandlerFunc {
	config := newHandlerConfig(options...)

	return func(ctx *Context) error {
		req := new(Req)
		if err := ctx.BindRequest(req); err != nil {
			return err
		}
		if err := ctx.Validate(req); err != nil {
			return err
		}
		if err := handler(ctx, req); err != nil {
			return err
		}

		return ctx.NoContent(config.emptyStatus)
	}
}

func HandleResponse[Res any](handler func(ctx *Context) (*Res, error), options ...HandlerOption) HandlerFunc {
	config := newHandlerConfig(options...)

	return func(ctx *Context) error {
		response, err := handler(ctx)
		if err != nil {
			return err
		}
		if response == nil {
			return ctx.NoContent(config.emptyStatus)
		}

		return ctx.JSON(config.successStatus, response)
	}
}

func HandleRequestResponse[Req any, Res any](
	handler func(ctx *Context, req *Req) (*Res, error),
	options ...HandlerOption,
) HandlerFunc {
	config := newHandlerConfig(options...)

	return func(ctx *Context) error {
		req := new(Req)
		if err := ctx.BindRequest(req); err != nil {
			return err
		}
		if err := ctx.Validate(req); err != nil {
			return err
		}

		response, err := handler(ctx, req)
		if err != nil {
			return err
		}
		if response == nil {
			return ctx.NoContent(config.emptyStatus)
		}

		return ctx.JSON(config.successStatus, response)
	}
}

func isContextResponseErrorHandler(handlerType reflect.Type) bool {
	return handlerType.Kind() == reflect.Func &&
		handlerType.NumIn() == 1 &&
		handlerType.In(0) == contextPointerType &&
		handlerType.NumOut() == 2 &&
		isPointerType(handlerType.Out(0)) &&
		handlerType.Out(1).Implements(errorType)
}

func isContextErrorHandler(handlerType reflect.Type) bool {
	return handlerType.Kind() == reflect.Func &&
		handlerType.NumIn() == 1 &&
		handlerType.In(0) == contextPointerType &&
		handlerType.NumOut() == 1 &&
		handlerType.Out(0).Implements(errorType)
}

func isRequestErrorHandler(handlerType reflect.Type) bool {
	return handlerType.Kind() == reflect.Func &&
		handlerType.NumIn() == 2 &&
		handlerType.In(0) == contextPointerType &&
		isPointerType(handlerType.In(1)) &&
		handlerType.NumOut() == 1 &&
		handlerType.Out(0).Implements(errorType)
}

func isRequestResponseErrorHandler(handlerType reflect.Type) bool {
	return handlerType.Kind() == reflect.Func &&
		handlerType.NumIn() == 2 &&
		handlerType.In(0) == contextPointerType &&
		isPointerType(handlerType.In(1)) &&
		handlerType.NumOut() == 2 &&
		isPointerType(handlerType.Out(0)) &&
		handlerType.Out(1).Implements(errorType)
}

func isPointerType(valueType reflect.Type) bool {
	return valueType.Kind() == reflect.Ptr
}

func callContextErrorHandler(handler reflect.Value, ctx *Context, config handlerConfig) error {
	results := handler.Call([]reflect.Value{reflect.ValueOf(ctx)})
	if err := errorValue(results[0]); err != nil {
		return err
	}

	return ctx.NoContent(config.emptyStatus)
}

func callContextResponseErrorHandler(handler reflect.Value, ctx *Context, config handlerConfig) error {
	results := handler.Call([]reflect.Value{reflect.ValueOf(ctx)})
	if err := errorValue(results[1]); err != nil {
		return err
	}

	response := results[0]
	if response.IsNil() {
		return ctx.NoContent(config.emptyStatus)
	}

	return ctx.JSON(config.successStatus, response.Interface())
}

func callRequestErrorHandler(handler reflect.Value, ctx *Context, config handlerConfig) error {
	request := reflect.New(handler.Type().In(1).Elem())
	if err := ctx.BindRequest(request.Interface()); err != nil {
		return err
	}
	if err := ctx.Validate(request.Interface()); err != nil {
		return err
	}

	results := handler.Call([]reflect.Value{reflect.ValueOf(ctx), request})
	if err := errorValue(results[0]); err != nil {
		return err
	}

	return ctx.NoContent(config.emptyStatus)
}

func callRequestResponseErrorHandler(handler reflect.Value, ctx *Context, config handlerConfig) error {
	request := reflect.New(handler.Type().In(1).Elem())
	if err := ctx.BindRequest(request.Interface()); err != nil {
		return err
	}
	if err := ctx.Validate(request.Interface()); err != nil {
		return err
	}

	results := handler.Call([]reflect.Value{reflect.ValueOf(ctx), request})
	if err := errorValue(results[1]); err != nil {
		return err
	}

	response := results[0]
	if response.IsNil() {
		return ctx.NoContent(config.emptyStatus)
	}

	return ctx.JSON(config.successStatus, response.Interface())
}

func errorValue(value reflect.Value) error {
	if value.IsNil() {
		return nil
	}

	err, ok := value.Interface().(error)
	if !ok {
		return Internal("handler returned a non-error value")
	}

	return err
}
