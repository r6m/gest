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

type containerConfig struct {
	value string
}

type containerLogger struct {
	name string
}

type containerFeatureService struct {
	config *containerConfig
	logger *containerLogger
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
	if !strings.Contains(err.Error(), "import a module that provides") {
		t.Fatalf("error = %q, want import-provides hint", err.Error())
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

func TestContainerImportedProviderIsVisible(t *testing.T) {
	imported := NewModule(ModuleConfig{
		Name: "ImportedModule",
		Providers: Providers(
			Provide(func() *containerRepository {
				return &containerRepository{name: "imported"}
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

func TestContainerNestedImportedProviderIsVisible(t *testing.T) {
	nested := NewModule(ModuleConfig{
		Name: "NestedModule",
		Providers: Providers(
			Provide(func() *containerRepository {
				return &containerRepository{name: "nested"}
			}),
		),
	})
	imported := NewModule(ModuleConfig{
		Name:    "ImportedModule",
		Imports: Imports(nested),
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
	if service.repository.name != "nested" {
		t.Fatalf("repository.name = %q, want %q", service.repository.name, "nested")
	}
}

func TestContainerDuplicateImportedProvidersReturnError(t *testing.T) {
	first := NewModule(ModuleConfig{
		Name: "FirstModule",
		Providers: Providers(
			Provide(func() *containerRepository {
				return &containerRepository{name: "first"}
			}),
		),
	})
	second := NewModule(ModuleConfig{
		Name: "SecondModule",
		Providers: Providers(
			Provide(func() *containerRepository {
				return &containerRepository{name: "second"}
			}),
		),
	})

	_, err := NewContainer(NewModule(ModuleConfig{
		Name:    "AppModule",
		Imports: Imports(first, second),
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

func TestContainerDirectGlobalProviderIsVisibleToUnrelatedModule(t *testing.T) {
	global := NewModule(ModuleConfig{
		Name:   "ConfigModule",
		Global: true,
		Providers: Providers(
			Value(&containerConfig{value: "app"}),
		),
	})
	feature := NewModule(ModuleConfig{
		Name: "FeatureModule",
		Providers: Providers(
			Provide(func(config *containerConfig) *containerFeatureService {
				return &containerFeatureService{config: config}
			}),
		),
	})
	container := newTestContainer(t, NewModule(ModuleConfig{
		Name:    "AppModule",
		Imports: Imports(global, feature),
	}))

	value, err := container.Resolve(TokenOf[*containerFeatureService]())
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	service, ok := value.(*containerFeatureService)
	if !ok {
		t.Fatalf("Resolve returned %T, want *containerFeatureService", value)
	}
	if service.config == nil || service.config.value != "app" {
		t.Fatalf("service.config = %#v, want app config", service.config)
	}
}

func TestContainerNestedGlobalProviderIsVisibleToUnrelatedModule(t *testing.T) {
	global := NewModule(ModuleConfig{
		Name:   "LoggerModule",
		Global: true,
		Providers: Providers(
			Value(&containerLogger{name: "global"}),
		),
	})
	settings := NewModule(ModuleConfig{
		Name:    "SettingsModule",
		Imports: Imports(global),
	})
	feature := NewModule(ModuleConfig{
		Name: "FeatureModule",
		Providers: Providers(
			Provide(func(logger *containerLogger) *containerFeatureService {
				return &containerFeatureService{logger: logger}
			}),
		),
	})
	container := newTestContainer(t, NewModule(ModuleConfig{
		Name:    "AppModule",
		Imports: Imports(settings, feature),
	}))

	value, err := container.Resolve(TokenOf[*containerFeatureService]())
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	service, ok := value.(*containerFeatureService)
	if !ok {
		t.Fatalf("Resolve returned %T, want *containerFeatureService", value)
	}
	if service.logger == nil || service.logger.name != "global" {
		t.Fatalf("service.logger = %#v, want global logger", service.logger)
	}
}

func TestContainerGlobalProviderMustBeExplicitlyImportedSomewhere(t *testing.T) {
	feature := NewModule(ModuleConfig{
		Name: "FeatureModule",
		Providers: Providers(
			Provide(func(config *containerConfig) *containerFeatureService {
				return &containerFeatureService{config: config}
			}),
		),
	})
	container := newTestContainer(t, NewModule(ModuleConfig{
		Name:    "AppModule",
		Imports: Imports(feature),
	}))

	_, err := container.Resolve(TokenOf[*containerFeatureService]())
	if err == nil {
		t.Fatal("Resolve returned nil error, want missing global provider error")
	}
	var diErr *diError
	if !errors.As(err, &diErr) {
		t.Fatalf("error type = %T, want *diError", err)
	}
	if diErr.Code != "DI_MISSING_PROVIDER" {
		t.Fatalf("Code = %q, want DI_MISSING_PROVIDER", diErr.Code)
	}
}

func TestContainerDuplicateGlobalProvidersReturnDeterministicError(t *testing.T) {
	first := NewModule(ModuleConfig{
		Name:   "FirstGlobalModule",
		Global: true,
		Providers: Providers(
			Value(&containerConfig{value: "first"}),
		),
	})
	second := NewModule(ModuleConfig{
		Name:   "SecondGlobalModule",
		Global: true,
		Providers: Providers(
			Value(&containerConfig{value: "second"}),
		),
	})

	_, err := NewContainer(NewModule(ModuleConfig{
		Name:    "AppModule",
		Imports: Imports(first, second),
	}))
	if err == nil {
		t.Fatal("NewContainer returned nil error, want duplicate global provider error")
	}
	var diErr *diError
	if !errors.As(err, &diErr) {
		t.Fatalf("error type = %T, want *diError", err)
	}
	if diErr.Code != "DI_DUPLICATE_PROVIDER" {
		t.Fatalf("Code = %q, want DI_DUPLICATE_PROVIDER", diErr.Code)
	}
	if diErr.Module != "AppModule" {
		t.Fatalf("Module = %q, want AppModule", diErr.Module)
	}
	if !strings.Contains(err.Error(), "SecondGlobalModule") || !strings.Contains(err.Error(), "FirstGlobalModule") {
		t.Fatalf("error = %q, want both conflicting global module names", err.Error())
	}
}

func TestContainerGlobalProviderConflictsWithLocalProvider(t *testing.T) {
	global := NewModule(ModuleConfig{
		Name:   "ConfigModule",
		Global: true,
		Providers: Providers(
			Value(&containerConfig{value: "global"}),
		),
	})
	feature := NewModule(ModuleConfig{
		Name: "FeatureModule",
		Providers: Providers(
			Value(&containerConfig{value: "local"}),
		),
	})

	_, err := NewContainer(NewModule(ModuleConfig{
		Name:    "AppModule",
		Imports: Imports(global, feature),
	}))
	if err == nil {
		t.Fatal("NewContainer returned nil error, want duplicate provider error")
	}
	if !strings.Contains(err.Error(), "FeatureModule") || !strings.Contains(err.Error(), "ConfigModule") {
		t.Fatalf("error = %q, want target and global module context", err.Error())
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

func TestContainerImportedAliasResolutionWorks(t *testing.T) {
	imported := NewModule(ModuleConfig{
		Name: "ImportedModule",
		Providers: Providers(
			Provide(func() *containerAliasImpl {
				return &containerAliasImpl{}
			}, As[containerAlias]()),
		),
	})
	container := newTestContainer(t, NewModule(ModuleConfig{
		Name:    "AppModule",
		Imports: Imports(imported),
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
