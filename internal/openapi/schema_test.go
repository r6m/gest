package openapi

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

type taggedDTO struct {
	ID       string `json:"id" validate:"required"`
	Name     string `json:"name,omitempty"`
	Age      int    `json:"age"`
	Secret   string `json:"-"`
	Fallback bool
}

type nestedDTO struct {
	Child childDTO `json:"child"`
}

type childDTO struct {
	CreatedAt time.Time `json:"createdAt"`
}

type collectionDTO struct {
	Names []string `json:"names"`
	Bytes [2]uint8 `json:"bytes"`
}

type pointerDTO struct {
	Name  *string   `json:"name,omitempty"`
	Child *childDTO `json:"child,omitempty"`
}

type recursiveDTO struct {
	ID   string        `json:"id"`
	Next *recursiveDTO `json:"next,omitempty"`
}

type unsupportedDTO struct {
	Lookup map[string]string `json:"lookup"`
}

func TestSchemaForPrimitiveTypes(t *testing.T) {
	tests := []struct {
		name string
		typ  reflect.Type
		want *Schema
	}{
		{name: "string", typ: reflect.TypeOf(""), want: &Schema{Type: "string"}},
		{name: "bool", typ: reflect.TypeOf(true), want: &Schema{Type: "boolean"}},
		{name: "int", typ: reflect.TypeOf(int64(0)), want: &Schema{Type: "integer"}},
		{name: "uint", typ: reflect.TypeOf(uint32(0)), want: &Schema{Type: "integer"}},
		{name: "float", typ: reflect.TypeOf(float64(0)), want: &Schema{Type: "number"}},
		{name: "time", typ: reflect.TypeOf(time.Time{}), want: &Schema{Type: "string", Format: "date-time"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SchemaForType(tt.typ)
			if err != nil {
				t.Fatalf("SchemaForType returned error: %v", err)
			}
			if !reflect.DeepEqual(result.Schema, tt.want) {
				t.Fatalf("schema = %#v, want %#v", result.Schema, tt.want)
			}
			if len(result.Components) != 0 {
				t.Fatalf("components = %#v, want empty", result.Components)
			}
		})
	}
}

func TestSchemaForStructUsesJSONTagsAndRequiredRules(t *testing.T) {
	result, err := SchemaFor((*taggedDTO)(nil))
	if err != nil {
		t.Fatalf("SchemaFor returned error: %v", err)
	}

	if result.Schema.Ref != "#/components/schemas/taggedDTO" {
		t.Fatalf("root ref = %q, want taggedDTO component ref", result.Schema.Ref)
	}
	schema := result.Components["taggedDTO"]
	if schema == nil {
		t.Fatal("missing taggedDTO component")
	}
	for _, name := range []string{"id", "name", "age", "Fallback"} {
		if schema.Properties[name] == nil {
			t.Fatalf("missing property %q in %#v", name, schema.Properties)
		}
	}
	if schema.Properties["Secret"] != nil || schema.Properties["-"] != nil {
		t.Fatalf("json:\"-\" field was not ignored: %#v", schema.Properties)
	}
	wantRequired := []string{"id", "age", "Fallback"}
	if !reflect.DeepEqual(schema.Required, wantRequired) {
		t.Fatalf("required = %#v, want %#v", schema.Required, wantRequired)
	}
}

func TestSchemaForValidateRequiredOverridesOmitEmpty(t *testing.T) {
	type dto struct {
		ID string `json:"id,omitempty" validate:"required"`
	}

	result, err := SchemaFor((*dto)(nil))
	if err != nil {
		t.Fatalf("SchemaFor returned error: %v", err)
	}

	required := result.Components["dto"].Required
	if !reflect.DeepEqual(required, []string{"id"}) {
		t.Fatalf("required = %#v, want id", required)
	}
}

func TestSchemaForNestedStructUsesStableComponents(t *testing.T) {
	result, err := SchemaFor((*nestedDTO)(nil))
	if err != nil {
		t.Fatalf("SchemaFor returned error: %v", err)
	}

	parent := result.Components["nestedDTO"]
	child := result.Components["childDTO"]
	if parent == nil || child == nil {
		t.Fatalf("components = %#v, want nestedDTO and childDTO", result.Components)
	}
	if parent.Properties["child"].Ref != "#/components/schemas/childDTO" {
		t.Fatalf("child schema = %#v, want childDTO ref", parent.Properties["child"])
	}
	if child.Properties["createdAt"].Type != "string" || child.Properties["createdAt"].Format != "date-time" {
		t.Fatalf("time schema = %#v, want date-time string", child.Properties["createdAt"])
	}
}

func TestSchemaForSlicesAndArrays(t *testing.T) {
	result, err := SchemaFor((*collectionDTO)(nil))
	if err != nil {
		t.Fatalf("SchemaFor returned error: %v", err)
	}

	schema := result.Components["collectionDTO"]
	if schema.Properties["names"].Type != "array" || schema.Properties["names"].Items.Type != "string" {
		t.Fatalf("names schema = %#v, want string array", schema.Properties["names"])
	}
	if schema.Properties["bytes"].Type != "array" || schema.Properties["bytes"].Items.Type != "integer" {
		t.Fatalf("bytes schema = %#v, want integer array", schema.Properties["bytes"])
	}
}

func TestSchemaForPointersMarksNullable(t *testing.T) {
	result, err := SchemaFor((*pointerDTO)(nil))
	if err != nil {
		t.Fatalf("SchemaFor returned error: %v", err)
	}

	schema := result.Components["pointerDTO"]
	if !schema.Properties["name"].Nullable || schema.Properties["name"].Type != "string" {
		t.Fatalf("name schema = %#v, want nullable string", schema.Properties["name"])
	}
	if !schema.Properties["child"].Nullable || schema.Properties["child"].Ref != "#/components/schemas/childDTO" {
		t.Fatalf("child schema = %#v, want nullable child ref", schema.Properties["child"])
	}
}

func TestSchemaForRecursiveStructUsesRef(t *testing.T) {
	result, err := SchemaFor((*recursiveDTO)(nil))
	if err != nil {
		t.Fatalf("SchemaFor returned error: %v", err)
	}

	next := result.Components["recursiveDTO"].Properties["next"]
	if next.Ref != "#/components/schemas/recursiveDTO" || !next.Nullable {
		t.Fatalf("next schema = %#v, want nullable recursive ref", next)
	}
}

func TestSchemaForProducesDeterministicJSON(t *testing.T) {
	first, err := SchemaFor((*nestedDTO)(nil))
	if err != nil {
		t.Fatalf("first SchemaFor returned error: %v", err)
	}
	second, err := SchemaFor((*nestedDTO)(nil))
	if err != nil {
		t.Fatalf("second SchemaFor returned error: %v", err)
	}

	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal first: %v", err)
	}
	secondJSON, err := json.Marshal(second)
	if err != nil {
		t.Fatalf("marshal second: %v", err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("schema output is not deterministic:\nfirst:  %s\nsecond: %s", firstJSON, secondJSON)
	}
}

func TestSchemaForUnsupportedTypesReturnsSchemaError(t *testing.T) {
	_, err := SchemaFor((*unsupportedDTO)(nil))
	if err == nil {
		t.Fatal("SchemaFor returned nil error, want unsupported type error")
	}
	var schemaErr *SchemaError
	if !errors.As(err, &schemaErr) {
		t.Fatalf("error = %T, want *SchemaError", err)
	}
	if !strings.Contains(err.Error(), "map[string]string") {
		t.Fatalf("error = %q, want unsupported map type", err.Error())
	}
}

func TestSchemaForNilMetadataReturnsSchemaError(t *testing.T) {
	_, err := SchemaFor(nil)
	if err == nil {
		t.Fatal("SchemaFor returned nil error, want missing metadata error")
	}
	var schemaErr *SchemaError
	if !errors.As(err, &schemaErr) {
		t.Fatalf("error = %T, want *SchemaError", err)
	}
}
