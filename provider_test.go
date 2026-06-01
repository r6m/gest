package gest

import (
	"reflect"
	"testing"
)

type providerTestService struct{}

type providerTestAlias interface {
	ProviderTestAlias()
}

func newProviderTestService() *providerTestService {
	return &providerTestService{}
}

func TestProvideDefaultsToServiceSingleton(t *testing.T) {
	provider := Provide(newProviderTestService)

	if provider.Kind != ProviderKindService {
		t.Fatalf("Kind = %q, want %q", provider.Kind, ProviderKindService)
	}
	if provider.Constructor == nil {
		t.Fatal("Constructor = nil, want constructor")
	}
	if provider.Value != nil {
		t.Fatalf("Value = %#v, want nil", provider.Value)
	}
	if provider.Scope != Singleton {
		t.Fatalf("Scope = %q, want %q", provider.Scope, Singleton)
	}
}

func TestControllerSetsControllerKind(t *testing.T) {
	provider := Controller(newProviderTestService)

	if provider.Kind != ProviderKindController {
		t.Fatalf("Kind = %q, want %q", provider.Kind, ProviderKindController)
	}
	if provider.Constructor == nil {
		t.Fatal("Constructor = nil, want constructor")
	}
	if provider.Scope != Singleton {
		t.Fatalf("Scope = %q, want %q", provider.Scope, Singleton)
	}
}

func TestValueSetsValueKindAndSingletonScope(t *testing.T) {
	value := &providerTestService{}

	provider := Value(value)

	if provider.Kind != ProviderKindValue {
		t.Fatalf("Kind = %q, want %q", provider.Kind, ProviderKindValue)
	}
	if provider.Value != value {
		t.Fatalf("Value = %#v, want %#v", provider.Value, value)
	}
	if provider.Constructor != nil {
		t.Fatalf("Constructor = %#v, want nil", provider.Constructor)
	}
	if provider.Scope != Singleton {
		t.Fatalf("Scope = %q, want %q", provider.Scope, Singleton)
	}
}

func TestExportMarksProviderExported(t *testing.T) {
	provider := Provide(newProviderTestService, Export())

	if !provider.Exported {
		t.Fatal("Exported = false, want true")
	}
}

func TestNameSetsName(t *testing.T) {
	provider := Provide(newProviderTestService, Name("service.main"))

	if provider.Name != "service.main" {
		t.Fatalf("Name = %q, want %q", provider.Name, "service.main")
	}
}

func TestWithScopeSetsScopeWithoutBehavior(t *testing.T) {
	provider := Provide(newProviderTestService, WithScope(Transient))

	if provider.Scope != Transient {
		t.Fatalf("Scope = %q, want %q", provider.Scope, Transient)
	}
}

func TestAsAddsTokenAlias(t *testing.T) {
	provider := Provide(newProviderTestService, As[providerTestAlias]())
	want := []Token{{Type: reflect.TypeFor[providerTestAlias]()}}

	if !reflect.DeepEqual(provider.Aliases, want) {
		t.Fatalf("Aliases = %#v, want %#v", provider.Aliases, want)
	}
}
