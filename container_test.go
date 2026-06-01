package gest

import (
	"errors"
	"strings"
	"testing"
)

type containerRepository struct {
	name string
}

type containerService struct {
	repository *containerRepository
}

type containerAlias interface {
	AliasName() string
}

type containerAliasImpl struct{}

func (containerAliasImpl) AliasName() string {
	return "alias"
}

type cycleA struct {
	b *cycleB
}

type cycleB struct {
	a *cycleA
}

func TestContainerResolveSimpleService(t *testing.T) {
	container := newTestContainer(t, NewModule(ModuleConfig{
		Name: "AppModule",
		Providers: Providers(
			Provide(func() *containerRepository {
				return &containerRepository{name: "main"}
			}),
		),
	}))

	value, err := container.Resolve(TokenOf[*containerRepository]())
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}

	repository, ok := value.(*containerRepository)
	if !ok {
		t.Fatalf("Resolve returned %T, want *containerRepository", value)
	}
	if repository.name != "main" {
		t.Fatalf("repository.name = %q, want %q", repository.name, "main")
	}
}

func TestContainerResolveServiceWithDependency(t *testing.T) {
	container := newTestContainer(t, NewModule(ModuleConfig{
		Name: "AppModule",
		Providers: Providers(
			Provide(func() *containerRepository {
				return &containerRepository{name: "main"}
			}),
			Provide(func(repository *containerRepository) *containerService {
				return &containerService{repository: repository}
			}),
		),
	}))

	value, err := container.Resolve(TokenOf[*containerService]())
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}

	service, ok := value.(*containerService)
	if !ok {
		t.Fatalf("Resolve returned %T, want *containerService", value)
	}
	if service.repository == nil {
		t.Fatal("service.repository = nil, want dependency")
	}
	if service.repository.name != "main" {
		t.Fatalf("service.repository.name = %q, want %q", service.repository.name, "main")
	}
}

func TestContainerCachesSingletonConstructors(t *testing.T) {
	calls := 0
	container := newTestContainer(t, NewModule(ModuleConfig{
		Name: "AppModule",
		Providers: Providers(
			Provide(func() *containerRepository {
				calls++
				return &containerRepository{}
			}),
		),
	}))

	first, err := container.Resolve(TokenOf[*containerRepository]())
	if err != nil {
		t.Fatalf("first Resolve returned error: %v", err)
	}
	second, err := container.Resolve(TokenOf[*containerRepository]())
	if err != nil {
		t.Fatalf("second Resolve returned error: %v", err)
	}

	if first != second {
		t.Fatal("Resolve returned different singleton instances")
	}
	if calls != 1 {
		t.Fatalf("constructor calls = %d, want 1", calls)
	}
}

func TestContainerResolveValueProvider(t *testing.T) {
	repository := &containerRepository{name: "value"}
	container := newTestContainer(t, NewModule(ModuleConfig{
		Name:      "AppModule",
		Providers: Providers(Value(repository)),
	}))

	value, err := container.Resolve(TokenOf[*containerRepository]())
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if value != repository {
		t.Fatalf("Resolve returned %#v, want %#v", value, repository)
	}
}

func TestContainerMissingProviderErrorIncludesTokenAndDependent(t *testing.T) {
	container := newTestContainer(t, NewModule(ModuleConfig{
		Name: "AppModule",
		Providers: Providers(
			Provide(func(repository *containerRepository) *containerService {
				return &containerService{repository: repository}
			}),
		),
	}))

	_, err := container.Resolve(TokenOf[*containerService]())
	if err == nil {
		t.Fatal("Resolve returned nil error, want missing provider error")
	}

	var diErr *diError
	if !errors.As(err, &diErr) {
		t.Fatalf("error type = %T, want *diError", err)
	}
	if diErr.Code != "DI_MISSING_PROVIDER" {
		t.Fatalf("Code = %q, want DI_MISSING_PROVIDER", diErr.Code)
	}
	if diErr.Token != TokenOf[*containerRepository]() {
		t.Fatalf("Token = %s, want %s", diErr.Token, TokenOf[*containerRepository]())
	}
	if diErr.Dependent != TokenOf[*containerService]() {
		t.Fatalf("Dependent = %s, want %s", diErr.Dependent, TokenOf[*containerService]())
	}
	if !strings.Contains(err.Error(), "missing provider") {
		t.Fatalf("error = %q, want missing provider context", err.Error())
	}
}

func TestContainerCycleErrorIncludesCyclePath(t *testing.T) {
	container := newTestContainer(t, NewModule(ModuleConfig{
		Name: "AppModule",
		Providers: Providers(
			Provide(func(b *cycleB) *cycleA {
				return &cycleA{b: b}
			}),
			Provide(func(a *cycleA) *cycleB {
				return &cycleB{a: a}
			}),
		),
	}))

	_, err := container.Resolve(TokenOf[*cycleA]())
	if err == nil {
		t.Fatal("Resolve returned nil error, want cycle error")
	}

	var diErr *diError
	if !errors.As(err, &diErr) {
		t.Fatalf("error type = %T, want *diError", err)
	}
	if diErr.Code != "DI_PROVIDER_CYCLE" {
		t.Fatalf("Code = %q, want DI_PROVIDER_CYCLE", diErr.Code)
	}
	wantPath := []Token{TokenOf[*cycleA](), TokenOf[*cycleB](), TokenOf[*cycleA]()}
	if len(diErr.Path) != len(wantPath) {
		t.Fatalf("Path length = %d, want %d", len(diErr.Path), len(wantPath))
	}
	for i := range wantPath {
		if diErr.Path[i] != wantPath[i] {
			t.Fatalf("Path[%d] = %s, want %s", i, diErr.Path[i], wantPath[i])
		}
	}
	if !strings.Contains(err.Error(), "provider cycle detected") {
		t.Fatalf("error = %q, want cycle context", err.Error())
	}
}

func TestContainerUnsupportedScopesReturnClearErrors(t *testing.T) {
	tests := []Scope{Request, Transient}

	for _, scope := range tests {
		t.Run(string(scope), func(t *testing.T) {
			_, err := NewContainer(NewModule(ModuleConfig{
				Name: "AppModule",
				Providers: Providers(
					Provide(func() *containerRepository {
						return &containerRepository{}
					}, WithScope(scope)),
				),
			}))
			if err == nil {
				t.Fatal("NewContainer returned nil error, want unsupported scope error")
			}

			var diErr *diError
			if !errors.As(err, &diErr) {
				t.Fatalf("error type = %T, want *diError", err)
			}
			if diErr.Code != "DI_UNSUPPORTED_SCOPE" {
				t.Fatalf("Code = %q, want DI_UNSUPPORTED_SCOPE", diErr.Code)
			}
			if !strings.Contains(err.Error(), "v0 supports singleton scope only") {
				t.Fatalf("error = %q, want singleton-only hint", err.Error())
			}
		})
	}
}

func TestContainerDuplicateProviderInSameModuleReturnsError(t *testing.T) {
	_, err := NewContainer(NewModule(ModuleConfig{
		Name: "AppModule",
		Providers: Providers(
			Provide(func() *containerRepository {
				return &containerRepository{name: "first"}
			}),
			Provide(func() *containerRepository {
				return &containerRepository{name: "second"}
			}),
		),
	}))
	if err == nil {
		t.Fatal("NewContainer returned nil error, want duplicate provider error")
	}

	var diErr *diError
	if !errors.As(err, &diErr) {
		t.Fatalf("error type = %T, want *diError", err)
	}
	if diErr.Code != "DI_DUPLICATE_PROVIDER" {
		t.Fatalf("Code = %q, want DI_DUPLICATE_PROVIDER", diErr.Code)
	}
}

func TestContainerNamedProvidersDisambiguateDuplicateTypes(t *testing.T) {
	container := newTestContainer(t, NewModule(ModuleConfig{
		Name: "AppModule",
		Providers: Providers(
			Provide(func() *containerRepository {
				return &containerRepository{name: "first"}
			}, Name("repo.first")),
			Provide(func() *containerRepository {
				return &containerRepository{name: "second"}
			}, Name("repo.second")),
		),
	}))

	first, err := container.Resolve(Named("repo.first"))
	if err != nil {
		t.Fatalf("Resolve first returned error: %v", err)
	}
	second, err := container.Resolve(Named("repo.second"))
	if err != nil {
		t.Fatalf("Resolve second returned error: %v", err)
	}
	firstRepository, ok := first.(*containerRepository)
	if !ok {
		t.Fatalf("Resolve first returned %T, want *containerRepository", first)
	}
	secondRepository, ok := second.(*containerRepository)
	if !ok {
		t.Fatalf("Resolve second returned %T, want *containerRepository", second)
	}
	if firstRepository.name != "first" {
		t.Fatalf("first repository name = %q, want first", firstRepository.name)
	}
	if secondRepository.name != "second" {
		t.Fatalf("second repository name = %q, want second", secondRepository.name)
	}
}

func TestContainerImportedExportedProviderIsVisible(t *testing.T) {
	imported := NewModule(ModuleConfig{
		Name: "ImportedModule",
		Providers: Providers(
			Provide(func() *containerRepository {
				return &containerRepository{name: "imported"}
			}, Export()),
		),
	})
	container := newTestContainer(t, NewModule(ModuleConfig{
		Name:    "AppModule",
		Imports: Imports(imported),
		Providers: Providers(
			Provide(func(repository *containerRepository) *containerService {
				return &containerService{repository: repository}
			}),
		),
	}))

	value, err := container.Resolve(TokenOf[*containerService]())
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	service, ok := value.(*containerService)
	if !ok {
		t.Fatalf("Resolve returned %T, want *containerService", value)
	}
	if service.repository.name != "imported" {
		t.Fatalf("repository.name = %q, want %q", service.repository.name, "imported")
	}
}

func TestContainerImportedUnexportedProviderIsNotVisible(t *testing.T) {
	imported := NewModule(ModuleConfig{
		Name: "ImportedModule",
		Providers: Providers(
			Provide(func() *containerRepository {
				return &containerRepository{}
			}),
		),
	})
	container := newTestContainer(t, NewModule(ModuleConfig{
		Name:    "AppModule",
		Imports: Imports(imported),
		Providers: Providers(
			Provide(func(repository *containerRepository) *containerService {
				return &containerService{repository: repository}
			}),
		),
	}))

	_, err := container.Resolve(TokenOf[*containerService]())
	if err == nil {
		t.Fatal("Resolve returned nil error, want missing provider error")
	}
	var diErr *diError
	if !errors.As(err, &diErr) {
		t.Fatalf("error type = %T, want *diError", err)
	}
	if diErr.Code != "DI_MISSING_PROVIDER" {
		t.Fatalf("Code = %q, want DI_MISSING_PROVIDER", diErr.Code)
	}
}

func TestContainerAliasResolutionWorks(t *testing.T) {
	container := newTestContainer(t, NewModule(ModuleConfig{
		Name: "AppModule",
		Providers: Providers(
			Provide(func() *containerAliasImpl {
				return &containerAliasImpl{}
			}, As[containerAlias]()),
		),
	}))

	value, err := container.Resolve(TokenOf[containerAlias]())
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	alias, ok := value.(containerAlias)
	if !ok {
		t.Fatalf("Resolve returned %T, want containerAlias", value)
	}
	if alias.AliasName() != "alias" {
		t.Fatalf("AliasName() = %q, want %q", alias.AliasName(), "alias")
	}
}

func TestContainerNamedProviderResolutionWorks(t *testing.T) {
	container := newTestContainer(t, NewModule(ModuleConfig{
		Name: "AppModule",
		Providers: Providers(
			Value(&containerRepository{name: "primary"}, Name("repo.primary")),
		),
	}))

	value, err := container.Resolve(Named("repo.primary"))
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	repository, ok := value.(*containerRepository)
	if !ok {
		t.Fatalf("Resolve returned %T, want *containerRepository", value)
	}
	if repository.name != "primary" {
		t.Fatalf("repository.name = %q, want %q", repository.name, "primary")
	}
}

func newTestContainer(t *testing.T, module Module) Container {
	t.Helper()

	container, err := NewContainer(module)
	if err != nil {
		t.Fatalf("NewContainer returned error: %v", err)
	}

	return container
}
