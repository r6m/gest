package gest

import (
	"reflect"
	"testing"
)

func TestNewModuleDefinitionMatchesInput(t *testing.T) {
	imported := NewModule(ModuleConfig{Name: "ImportedModule"})
	provider := struct{ name string }{name: "provider"}

	mod := NewModule(ModuleConfig{
		Name:      "ReportsModule",
		Global:    true,
		Imports:   []Module{imported},
		Providers: []Provider{provider},
	})

	got := mod.Definition()
	if got.Name != "ReportsModule" {
		t.Fatalf("Name = %q, want %q", got.Name, "ReportsModule")
	}
	if !got.Global {
		t.Fatal("Global = false, want true")
	}
	if !reflect.DeepEqual(got.Imports, []Module{imported}) {
		t.Fatalf("Imports = %#v, want %#v", got.Imports, []Module{imported})
	}
	if !reflect.DeepEqual(got.Providers, []Provider{provider}) {
		t.Fatalf("Providers = %#v, want %#v", got.Providers, []Provider{provider})
	}
}

func TestImportsPreservesOrder(t *testing.T) {
	first := NewModule(ModuleConfig{Name: "FirstModule"})
	second := NewModule(ModuleConfig{Name: "SecondModule"})

	got := Imports(first, second)

	if !reflect.DeepEqual(got, []Module{first, second}) {
		t.Fatalf("Imports order = %#v, want %#v", got, []Module{first, second})
	}
}

func TestProvidersPreservesOrder(t *testing.T) {
	first := struct{ name string }{name: "first"}
	second := struct{ name string }{name: "second"}

	got := Providers(first, second)

	if !reflect.DeepEqual(got, []Provider{first, second}) {
		t.Fatalf("Providers order = %#v, want %#v", got, []Provider{first, second})
	}
}

func TestDefinitionDoesNotExposeInternalSlices(t *testing.T) {
	firstImport := NewModule(ModuleConfig{Name: "FirstImport"})
	secondImport := NewModule(ModuleConfig{Name: "SecondImport"})
	firstProvider := struct{ name string }{name: "first"}
	secondProvider := struct{ name string }{name: "second"}

	mod := NewModule(ModuleConfig{
		Name:      "ReportsModule",
		Imports:   []Module{firstImport},
		Providers: []Provider{firstProvider},
	})

	definition := mod.Definition()
	definition.Imports[0] = secondImport
	definition.Providers[0] = secondProvider

	got := mod.Definition()
	if !reflect.DeepEqual(got.Imports, []Module{firstImport}) {
		t.Fatalf("Imports were mutated through Definition: got %#v, want %#v", got.Imports, []Module{firstImport})
	}
	if !reflect.DeepEqual(got.Providers, []Provider{firstProvider}) {
		t.Fatalf("Providers were mutated through Definition: got %#v, want %#v", got.Providers, []Provider{firstProvider})
	}
}

func TestNewModuleCopiesInputSlices(t *testing.T) {
	firstImport := NewModule(ModuleConfig{Name: "FirstImport"})
	secondImport := NewModule(ModuleConfig{Name: "SecondImport"})
	firstProvider := struct{ name string }{name: "first"}
	secondProvider := struct{ name string }{name: "second"}
	imports := []Module{firstImport}
	providers := []Provider{firstProvider}

	mod := NewModule(ModuleConfig{
		Name:      "ReportsModule",
		Imports:   imports,
		Providers: providers,
	})
	imports[0] = secondImport
	providers[0] = secondProvider

	got := mod.Definition()
	if !reflect.DeepEqual(got.Imports, []Module{firstImport}) {
		t.Fatalf("Imports were mutated through input slice: got %#v, want %#v", got.Imports, []Module{firstImport})
	}
	if !reflect.DeepEqual(got.Providers, []Provider{firstProvider}) {
		t.Fatalf("Providers were mutated through input slice: got %#v, want %#v", got.Providers, []Provider{firstProvider})
	}
}
