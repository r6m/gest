package openapi

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

var timeType = reflect.TypeOf(time.Time{})

// Schema is a minimal OpenAPI-compatible schema representation.
type Schema struct {
	Ref        string             `json:"$ref,omitempty"`
	Type       string             `json:"type,omitempty"`
	Format     string             `json:"format,omitempty"`
	Nullable   bool               `json:"nullable,omitempty"`
	Items      *Schema            `json:"items,omitempty"`
	Properties map[string]*Schema `json:"properties,omitempty"`
	Required   []string           `json:"required,omitempty"`
}

// Result contains a schema and any named struct components it references.
type Result struct {
	Schema     *Schema
	Components map[string]*Schema
}

// SchemaFor generates a minimal schema from explicit DTO metadata.
func SchemaFor(metadata any) (*Result, error) {
	if metadata == nil {
		return nil, &SchemaError{Type: "<nil>", Reason: "missing schema metadata"}
	}
	return SchemaForType(reflect.TypeOf(metadata))
}

// SchemaForType generates a minimal schema from a reflected DTO type.
func SchemaForType(typ reflect.Type) (*Result, error) {
	if typ == nil {
		return nil, &SchemaError{Type: "<nil>", Reason: "missing schema type"}
	}
	generator := &schemaGenerator{
		components: make(map[string]*Schema),
		names:      make(map[reflect.Type]string),
		building:   make(map[reflect.Type]bool),
	}
	schema, err := generator.schemaFor(typ)
	if err != nil {
		return nil, err
	}
	return &Result{
		Schema:     schema,
		Components: generator.components,
	}, nil
}

// SchemaError describes a DTO type that cannot be represented by the MVP schema generator.
type SchemaError struct {
	Type   string
	Reason string
}

func (e *SchemaError) Error() string {
	return "openapi schema: " + e.Type + ": " + e.Reason
}

type schemaGenerator struct {
	components map[string]*Schema
	names      map[reflect.Type]string
	building   map[reflect.Type]bool
}

func (g *schemaGenerator) schemaFor(typ reflect.Type) (*Schema, error) {
	if typ.Kind() == reflect.Pointer {
		element := typ
		for element.Kind() == reflect.Pointer {
			element = element.Elem()
		}
		schema, err := g.schemaFor(element)
		if err != nil {
			return nil, err
		}
		clone := cloneSchema(schema)
		clone.Nullable = true
		return clone, nil
	}

	if typ == timeType {
		return &Schema{Type: "string", Format: "date-time"}, nil
	}

	switch typ.Kind() {
	case reflect.String:
		return &Schema{Type: "string"}, nil
	case reflect.Bool:
		return &Schema{Type: "boolean"}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return &Schema{Type: "integer"}, nil
	case reflect.Float32, reflect.Float64:
		return &Schema{Type: "number"}, nil
	case reflect.Slice, reflect.Array:
		items, err := g.schemaFor(typ.Elem())
		if err != nil {
			return nil, err
		}
		return &Schema{Type: "array", Items: items}, nil
	case reflect.Struct:
		return g.structSchema(typ)
	case reflect.Map, reflect.Func, reflect.Chan, reflect.Interface, reflect.UnsafePointer, reflect.Complex64, reflect.Complex128:
		return nil, unsupportedTypeError(typ)
	default:
		return nil, unsupportedTypeError(typ)
	}
}

func (g *schemaGenerator) structSchema(typ reflect.Type) (*Schema, error) {
	if typ.Name() == "" {
		return g.objectSchema(typ)
	}

	name := g.componentName(typ)
	if _, ok := g.components[name]; ok {
		return refSchema(name), nil
	}
	if g.building[typ] {
		return refSchema(name), nil
	}

	g.building[typ] = true
	g.components[name] = &Schema{Type: "object"}
	object, err := g.objectSchema(typ)
	if err != nil {
		delete(g.components, name)
		delete(g.building, typ)
		return nil, err
	}
	g.components[name] = object
	delete(g.building, typ)

	return refSchema(name), nil
}

func (g *schemaGenerator) objectSchema(typ reflect.Type) (*Schema, error) {
	object := &Schema{
		Type:       "object",
		Properties: make(map[string]*Schema),
	}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}

		name, omitEmpty, ignored := jsonFieldName(field)
		if ignored {
			continue
		}

		schema, err := g.schemaFor(field.Type)
		if err != nil {
			return nil, err
		}
		object.Properties[name] = schema
		if !omitEmpty || hasRequiredValidation(field) {
			object.Required = append(object.Required, name)
		}
	}
	if len(object.Properties) == 0 {
		object.Properties = nil
	}
	if len(object.Required) == 0 {
		object.Required = nil
	}
	return object, nil
}

func (g *schemaGenerator) componentName(typ reflect.Type) string {
	if name, ok := g.names[typ]; ok {
		return name
	}

	base := typ.Name()
	name := base
	index := 2
	for {
		available := true
		for existingType, existingName := range g.names {
			if existingName == name && existingType != typ {
				available = false
				break
			}
		}
		if available {
			g.names[typ] = name
			return name
		}
		name = fmt.Sprintf("%s%d", base, index)
		index++
	}
}

func jsonFieldName(field reflect.StructField) (name string, omitEmpty bool, ignored bool) {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return "", false, true
	}
	parts := strings.Split(tag, ",")
	if parts[0] == "" {
		name = field.Name
	} else {
		name = parts[0]
	}
	for _, option := range parts[1:] {
		if option == "omitempty" {
			omitEmpty = true
			break
		}
	}
	return name, omitEmpty, false
}

func hasRequiredValidation(field reflect.StructField) bool {
	for _, rule := range strings.Split(field.Tag.Get("validate"), ",") {
		if strings.TrimSpace(rule) == "required" {
			return true
		}
	}
	return false
}

func refSchema(name string) *Schema {
	return &Schema{Ref: "#/components/schemas/" + name}
}

func unsupportedTypeError(typ reflect.Type) error {
	return &SchemaError{Type: typ.String(), Reason: "unsupported type"}
}

func cloneSchema(schema *Schema) *Schema {
	if schema == nil {
		return nil
	}
	clone := *schema
	if schema.Items != nil {
		clone.Items = cloneSchema(schema.Items)
	}
	if schema.Properties != nil {
		clone.Properties = make(map[string]*Schema, len(schema.Properties))
		for name, property := range schema.Properties {
			clone.Properties[name] = cloneSchema(property)
		}
	}
	clone.Required = append([]string(nil), schema.Required...)
	return &clone
}
