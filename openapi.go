package gest

// OpenAPIRoute describes route metadata collected by runtime bootstrap.
//
// It is intentionally a route catalog, not a generated OpenAPI operation.
// Schema generation and serving an OpenAPI document are owned by later phases.
type OpenAPIRoute struct {
	ControllerName string
	Tag            string
	BasePath       string
	RouteName      string
	Method         string
	Path           string
	Statuses       []int
	Summary        string
	Description    string
	Request        any
	Response       any
}

func newOpenAPIRoute(controller ControllerDefinition, route RouteDefinition, fullPath string) OpenAPIRoute {
	return OpenAPIRoute{
		ControllerName: controller.Name,
		Tag:            controller.Tag,
		BasePath:       controller.BasePath,
		RouteName:      route.Name,
		Method:         route.Method,
		Path:           fullPath,
		Statuses:       append([]int(nil), route.Statuses...),
		Summary:        route.Metadata.Summary,
		Description:    route.Metadata.Description,
		Request:        route.Request,
		Response:       route.Response,
	}
}

func cloneOpenAPIRoutes(routes []OpenAPIRoute) []OpenAPIRoute {
	cloned := append([]OpenAPIRoute(nil), routes...)
	for i := range cloned {
		cloned[i].Statuses = append([]int(nil), cloned[i].Statuses...)
	}
	return cloned
}
