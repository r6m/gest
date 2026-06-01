package gest

import (
	"fmt"
	"reflect"
)

var (
	contextPointerType = reflect.TypeOf((*Context)(nil))
	errorType          = reflect.TypeOf((*error)(nil)).Elem()
)

// JSON wraps a typed JSON controller handler as a runtime HandlerFunc.
func JSON(handler any, options ...HandlerOption) HandlerFunc {
	config := newHandlerConfig(options...)
	handlerValue := reflect.ValueOf(handler)
	if !handlerValue.IsValid() {
		return func(ctx *Context) error {
			return Internal("unsupported JSON handler signature <nil>")
		}
	}
	if handlerValue.Kind() == reflect.Func && handlerValue.IsNil() {
		return func(ctx *Context) error {
			return Internal("unsupported JSON handler signature <nil>")
		}
	}
	handlerType := handlerValue.Type()

	return func(ctx *Context) error {
		switch {
		case isContextErrorHandler(handlerType):
			return callContextErrorHandler(handlerValue, ctx, config)
		case isRequestErrorHandler(handlerType):
			return callRequestErrorHandler(handlerValue, ctx, config)
		case isRequestResponseErrorHandler(handlerType):
			return callRequestResponseErrorHandler(handlerValue, ctx, config)
		default:
			return Internal(fmt.Sprintf("unsupported JSON handler signature %s", handlerType))
		}
	}
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

func callRequestErrorHandler(handler reflect.Value, ctx *Context, config handlerConfig) error {
	request := reflect.New(handler.Type().In(1).Elem())
	if err := ctx.BindRequest(request.Interface()); err != nil {
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
		return Internal("JSON handler returned a non-error value")
	}

	return err
}
