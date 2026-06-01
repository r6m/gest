package gest

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	schemagen "github.com/r6m/gest/internal/openapi"
)

type openAPIDocument struct {
	OpenAPI    string                 `json:"openapi"`
	Info       openAPIInfo            `json:"info"`
	Paths      map[string]openAPIPath `json:"paths"`
	Components openAPIComponents      `json:"components,omitempty"`
}

type openAPIInfo struct {
	Title   string `json:"title"`
	Version string `json:"version"`
}

type openAPIPath map[string]openAPIOperation

type openAPIOperation struct {
	OperationID string                     `json:"operationId"`
	Tags        []string                   `json:"tags,omitempty"`
	Summary     string                     `json:"summary,omitempty"`
	Description string                     `json:"description,omitempty"`
	Parameters  []openAPIParameter         `json:"parameters,omitempty"`
	RequestBody *openAPIRequestBody        `json:"requestBody,omitempty"`
	Responses   map[string]openAPIResponse `json:"responses"`
}

type openAPIParameter struct {
	Name     string            `json:"name"`
	In       string            `json:"in"`
	Required bool              `json:"required,omitempty"`
	Schema   *schemagen.Schema `json:"schema"`
}

type openAPIRequestBody struct {
	Required bool                        `json:"required,omitempty"`
	Content  map[string]openAPIMediaType `json:"content"`
}

type openAPIMediaType struct {
	Schema *schemagen.Schema `json:"schema"`
}

type openAPIResponse struct {
	Description string                      `json:"description"`
	Content     map[string]openAPIMediaType `json:"content,omitempty"`
}

type openAPIComponents struct {
	Schemas map[string]*schemagen.Schema `json:"schemas,omitempty"`
}

func buildOpenAPIDocument(config openAPIConfig, routes []OpenAPIRoute) (*openAPIDocument, error) {
	builder := openAPIDocumentBuilder{
		document: &openAPIDocument{
			OpenAPI: "3.0.3",
			Info: openAPIInfo{
				Title:   config.Title,
				Version: config.Version,
			},
			Paths: make(map[string]openAPIPath),
			Components: openAPIComponents{
				Schemas: make(map[string]*schemagen.Schema),
			},
		},
	}
	for _, route := range routes {
		operation, err := builder.operation(route)
		if err != nil {
			return nil, err
		}
		pathItem := builder.document.Paths[route.Path]
		if pathItem == nil {
			pathItem = make(openAPIPath)
			builder.document.Paths[route.Path] = pathItem
		}
		pathItem[strings.ToLower(route.Method)] = operation
	}
	if len(builder.document.Components.Schemas) == 0 {
		builder.document.Components.Schemas = nil
	}
	return builder.document, nil
}

type openAPIDocumentBuilder struct {
	document *openAPIDocument
}

func (b *openAPIDocumentBuilder) operation(route OpenAPIRoute) (openAPIOperation, error) {
	operation := openAPIOperation{
		OperationID: routeOperationID(route),
		Summary:     route.Summary,
		Description: route.Description,
		Responses:   make(map[string]openAPIResponse),
	}
	if route.Tag != "" {
		operation.Tags = []string{route.Tag}
	}
	if route.Request != nil {
		request, err := b.request(route.Request)
		if err != nil {
			return openAPIOperation{}, err
		}
		operation.Parameters = request.parameters
		operation.RequestBody = request.body
	}
	statuses := route.Statuses
	if len(statuses) == 0 {
		statuses = []int{http.StatusOK}
	}
	for _, status := range statuses {
		response := openAPIResponse{Description: http.StatusText(status)}
		if route.Response != nil && status != http.StatusNoContent {
			schema, err := b.schemaFor(route.Response)
			if err != nil {
				return openAPIOperation{}, err
			}
			response.Content = jsonContent(schema)
		}
		operation.Responses[strconv.Itoa(status)] = response
	}
	return operation, nil
}

type openAPIRequest struct {
	parameters []openAPIParameter
	body       *openAPIRequestBody
}

func (b *openAPIDocumentBuilder) request(metadata any) (openAPIRequest, error) {
	if _, err := b.schemaFor(metadata); err != nil {
		return openAPIRequest{}, err
	}

	typ := reflect.TypeOf(metadata)
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return openAPIRequest{}, nil
	}

	var request openAPIRequest
	body := &schemagen.Schema{Type: "object", Properties: make(map[string]*schemagen.Schema)}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}
		if name, ok := openAPITagName(field.Tag.Get("param")); ok {
			schema, err := b.schemaForType(field.Type)
			if err != nil {
				return openAPIRequest{}, err
			}
			request.parameters = append(request.parameters, openAPIParameter{Name: name, In: "path", Required: true, Schema: schema})
		}
		if name, ok := openAPITagName(field.Tag.Get("query")); ok {
			schema, err := b.schemaForType(field.Type)
			if err != nil {
				return openAPIRequest{}, err
			}
			request.parameters = append(request.parameters, openAPIParameter{Name: name, In: "query", Required: hasRequiredValidation(field), Schema: schema})
		}
		if name, ok := openAPITagName(field.Tag.Get("header")); ok {
			schema, err := b.schemaForType(field.Type)
			if err != nil {
				return openAPIRequest{}, err
			}
			request.parameters = append(request.parameters, openAPIParameter{Name: name, In: "header", Required: hasRequiredValidation(field), Schema: schema})
		}
		if name, omitEmpty, ok := openAPIJSONName(field); ok {
			schema, err := b.schemaForType(field.Type)
			if err != nil {
				return openAPIRequest{}, err
			}
			body.Properties[name] = schema
			if !omitEmpty || hasRequiredValidation(field) {
				body.Required = append(body.Required, name)
			}
		}
	}
	if len(body.Properties) > 0 {
		request.body = &openAPIRequestBody{
			Required: len(body.Required) > 0,
			Content:  jsonContent(body),
		}
	}
	return request, nil
}

func (b *openAPIDocumentBuilder) schemaFor(metadata any) (*schemagen.Schema, error) {
	result, err := schemagen.SchemaFor(metadata)
	if err != nil {
		return nil, err
	}
	b.mergeComponents(result.Components)
	return result.Schema, nil
}

func (b *openAPIDocumentBuilder) schemaForType(typ reflect.Type) (*schemagen.Schema, error) {
	result, err := schemagen.SchemaForType(typ)
	if err != nil {
		return nil, err
	}
	b.mergeComponents(result.Components)
	return result.Schema, nil
}

func (b *openAPIDocumentBuilder) mergeComponents(components map[string]*schemagen.Schema) {
	for name, schema := range components {
		b.document.Components.Schemas[name] = schema
	}
}

func jsonContent(schema *schemagen.Schema) map[string]openAPIMediaType {
	return map[string]openAPIMediaType{
		"application/json": {Schema: schema},
	}
}

func routeOperationID(route OpenAPIRoute) string {
	if route.ControllerName == "" {
		return route.RouteName
	}
	if route.RouteName == "" {
		return route.ControllerName
	}
	return route.ControllerName + "." + route.RouteName
}

func openAPITagName(tag string) (string, bool) {
	if tag == "" || tag == "-" {
		return "", false
	}
	name, _, _ := strings.Cut(tag, ",")
	return name, name != ""
}

func openAPIJSONName(field reflect.StructField) (string, bool, bool) {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return "", false, false
	}
	name, options, _ := strings.Cut(tag, ",")
	if name == "" {
		return "", false, false
	}
	return name, strings.Contains(","+options+",", ",omitempty,"), true
}

func hasRequiredValidation(field reflect.StructField) bool {
	for _, rule := range strings.Split(field.Tag.Get("validate"), ",") {
		if strings.TrimSpace(rule) == "required" {
			return true
		}
	}
	return false
}

func writeOpenAPIDocument(ctx *Context, document *openAPIDocument) error {
	ctx.RawResponse().Header().Set("Content-Type", "application/json")
	ctx.RawResponse().WriteHeader(http.StatusOK)
	return json.NewEncoder(ctx.RawResponse()).Encode(document)
}
