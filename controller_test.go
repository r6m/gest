package gest

import "testing"

type fakeController struct{}

func (fakeController) GestController() ControllerDefinition {
	return ControllerDefinition{
		Name:     "FakeController",
		BasePath: "/fake",
		Tag:      "Fake",
		Routes: []RouteDefinition{
			{
				Name:   "First",
				Method: "GET",
				Path:   "/first",
			},
			{
				Name:   "Second",
				Method: "POST",
				Path:   "/second",
			},
		},
	}
}

func TestFakeControllerSatisfiesDescribedController(t *testing.T) {
	var controller DescribedController = fakeController{}

	definition := controller.GestController()
	if definition.Name != "FakeController" {
		t.Fatalf("Name = %q, want %q", definition.Name, "FakeController")
	}
	if definition.BasePath != "/fake" {
		t.Fatalf("BasePath = %q, want %q", definition.BasePath, "/fake")
	}
}

func TestControllerDefinitionPreservesRouteOrder(t *testing.T) {
	definition := fakeController{}.GestController()

	if len(definition.Routes) != 2 {
		t.Fatalf("Routes length = %d, want 2", len(definition.Routes))
	}
	if definition.Routes[0].Name != "First" {
		t.Fatalf("Routes[0].Name = %q, want %q", definition.Routes[0].Name, "First")
	}
	if definition.Routes[1].Name != "Second" {
		t.Fatalf("Routes[1].Name = %q, want %q", definition.Routes[1].Name, "Second")
	}
}

func TestEmptyRouteDefinitionsAreAllowedAtTypeLevel(t *testing.T) {
	definition := ControllerDefinition{
		Name: "EmptyController",
		Routes: []RouteDefinition{
			{},
		},
	}

	if len(definition.Routes) != 1 {
		t.Fatalf("Routes length = %d, want 1", len(definition.Routes))
	}
	if definition.Routes[0].Method != "" {
		t.Fatalf("empty route Method = %q, want empty", definition.Routes[0].Method)
	}
}
