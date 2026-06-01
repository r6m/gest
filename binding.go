package gest

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
)

// BindRequest binds JSON body, path params, query params, and headers into v.
func (c *Context) BindRequest(v any) error {
	target := reflect.ValueOf(v)
	if !target.IsValid() || target.Kind() != reflect.Ptr || target.IsNil() {
		return BadRequest("BindRequest target must be a non-nil pointer to a struct")
	}

	structValue := target.Elem()
	if structValue.Kind() != reflect.Struct {
		return BadRequest("BindRequest target must be a pointer to a struct")
	}

	fields := bindableFields(structValue.Type())
	if err := c.bindJSONBody(structValue, fields); err != nil {
		return err
	}
	if err := c.bindPathParams(structValue, fields); err != nil {
		return err
	}
	if err := c.bindQueryParams(structValue, fields); err != nil {
		return err
	}

	return c.bindHeaders(structValue, fields)
}

type bindField struct {
	index        int
	name         string
	defaultValue string
	hasDefault   bool
}

type bindFields struct {
	json    []bindField
	params  []bindField
	queries []bindField
	headers []bindField
}

func bindableFields(structType reflect.Type) bindFields {
	var fields bindFields
	for index := range structType.NumField() {
		field := structType.Field(index)
		if field.PkgPath != "" {
			continue
		}

		defaultValue, hasDefault := field.Tag.Lookup("default")
		if name, ok := tagName(field.Tag.Get("json")); ok {
			fields.json = append(fields.json, bindField{index: index, name: name, defaultValue: defaultValue, hasDefault: hasDefault})
		}
		if name, ok := tagName(field.Tag.Get("param")); ok {
			fields.params = append(fields.params, bindField{index: index, name: name, defaultValue: defaultValue, hasDefault: hasDefault})
		}
		if name, ok := tagName(field.Tag.Get("query")); ok {
			fields.queries = append(fields.queries, bindField{index: index, name: name, defaultValue: defaultValue, hasDefault: hasDefault})
		}
		if name, ok := tagName(field.Tag.Get("header")); ok {
			fields.headers = append(fields.headers, bindField{index: index, name: name, defaultValue: defaultValue, hasDefault: hasDefault})
		}
	}

	return fields
}

func tagName(tag string) (string, bool) {
	if tag == "" || tag == "-" {
		return "", false
	}

	name, _, _ := strings.Cut(tag, ",")
	if name == "" {
		return "", false
	}

	return name, true
}

func (c *Context) bindJSONBody(target reflect.Value, fields bindFields) error {
	if len(fields.json) == 0 || c.request.Body == nil {
		return nil
	}

	body, err := io.ReadAll(c.request.Body)
	if err != nil {
		return BadRequest("read request body: " + err.Error())
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return nil
	}

	var values map[string]json.RawMessage
	if err := json.Unmarshal(body, &values); err != nil {
		return bindingError(
			"BINDING_MALFORMED_JSON",
			"request body must be valid JSON",
			"body",
			"Send a valid JSON object body.",
		)
	}

	for _, bindField := range fields.json {
		raw, ok := values[bindField.name]
		if !ok {
			continue
		}

		field := target.Field(bindField.index)
		if !field.CanSet() {
			continue
		}

		if err := json.Unmarshal(raw, field.Addr().Interface()); err != nil {
			return conversionError("json."+bindField.name, string(raw), field.Type())
		}
	}

	return nil
}

func (c *Context) bindPathParams(target reflect.Value, fields bindFields) error {
	for _, bindField := range fields.params {
		value := c.Param(bindField.name)
		if value == "" {
			return bindingError(
				"BINDING_MISSING_REQUIRED_PARAM",
				fmt.Sprintf("missing path parameter %q", bindField.name),
				"param."+bindField.name,
				"",
			)
		}

		if err := setScalarField(target.Field(bindField.index), value); err != nil {
			return conversionError("param."+bindField.name, value, target.Field(bindField.index).Type())
		}
	}

	return nil
}

func (c *Context) bindQueryParams(target reflect.Value, fields bindFields) error {
	for _, bindField := range fields.queries {
		value := c.Query(bindField.name)
		usedDefault := false
		if value == "" {
			if !bindField.hasDefault {
				continue
			}
			value = bindField.defaultValue
			usedDefault = true
		}

		if err := setScalarField(target.Field(bindField.index), value); err != nil {
			if usedDefault {
				return defaultConversionError("query."+bindField.name, value, target.Field(bindField.index).Type())
			}
			return conversionError("query."+bindField.name, value, target.Field(bindField.index).Type())
		}
	}

	return nil
}

func (c *Context) bindHeaders(target reflect.Value, fields bindFields) error {
	for _, bindField := range fields.headers {
		value := c.Header(bindField.name)
		usedDefault := false
		if value == "" {
			if !bindField.hasDefault {
				continue
			}
			value = bindField.defaultValue
			usedDefault = true
		}

		if err := setScalarField(target.Field(bindField.index), value); err != nil {
			if usedDefault {
				return defaultConversionError("header."+bindField.name, value, target.Field(bindField.index).Type())
			}
			return conversionError("header."+bindField.name, value, target.Field(bindField.index).Type())
		}
	}

	return nil
}

func setScalarField(field reflect.Value, value string) error {
	if !field.CanSet() {
		return errors.New("field cannot be set")
	}

	if field.Kind() == reflect.Ptr {
		next := reflect.New(field.Type().Elem())
		if err := setScalarField(next.Elem(), value); err != nil {
			return err
		}
		field.Set(next)
		return nil
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Bool:
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		field.SetBool(parsed)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		parsed, err := strconv.ParseInt(value, 10, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetInt(parsed)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		parsed, err := strconv.ParseUint(value, 10, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetUint(parsed)
	case reflect.Float32, reflect.Float64:
		parsed, err := strconv.ParseFloat(value, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetFloat(parsed)
	default:
		return fmt.Errorf("unsupported scalar type %s", field.Type())
	}

	return nil
}

func conversionError(field string, value string, target reflect.Type) error {
	return bindingError(
		"BINDING_CONVERSION_FAILURE",
		fmt.Sprintf("%s value %q cannot be converted to %s", field, value, target),
		field,
		conversionHint(target),
	)
}

func defaultConversionError(field string, value string, target reflect.Type) error {
	return bindingError(
		"BINDING_CONVERSION_FAILURE",
		fmt.Sprintf("%s default value %q cannot be converted to %s", field, value, target),
		field,
		conversionHint(target),
	)
}

func conversionHint(target reflect.Type) string {
	if target.Kind() == reflect.Ptr {
		target = target.Elem()
	}

	switch target.Kind() {
	case reflect.Bool:
		return "Use true or false."
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "Use a base-10 signed integer value."
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return "Use a base-10 unsigned integer value."
	case reflect.Float32, reflect.Float64:
		return "Use a numeric value."
	default:
		return ""
	}
}
