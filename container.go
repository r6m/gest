package gest

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
)

// Container resolves providers from a module graph.
type Container interface {
	Resolve(token Token) (any, error)
	MustResolve(token Token) any
	Invoke(constructor any) (any, error)
}

// NewContainer builds a singleton DI container for a root module.
func NewContainer(root Module) (Container, error) {
	builder := containerBuilder{}

	rootContainer, err := builder.build(root)
	if err != nil {
		return nil, err
	}

	return &container{root: rootContainer}, nil
}

type container struct {
	root *moduleContainer
}

func (c *container) Resolve(token Token) (any, error) {
	provider, ok := c.root.visible[token]
	if !ok {
		return nil, missingProviderError(token, Token{}, c.root.name)
	}

	return provider.resolve(nil)
}

func (c *container) MustResolve(token Token) any {
	value, err := c.Resolve(token)
	if err != nil {
		panic(err)
	}

	return value
}

func (c *container) Invoke(constructor any) (any, error) {
	return invokeInModule(c.root, constructor, Token{})
}

type containerBuilder struct{}

func (b *containerBuilder) build(mod Module) (*moduleContainer, error) {
	definition := mod.Definition()
	module := &moduleContainer{
		name:     definition.Name,
		imports:  make([]*moduleContainer, 0, len(definition.Imports)),
		own:      make(map[Token]*providerState),
		visible:  make(map[Token]*providerState),
		exported: make(map[Token]*providerState),
	}

	for _, imported := range definition.Imports {
		importedModule, err := b.build(imported)
		if err != nil {
			return nil, err
		}
		module.imports = append(module.imports, importedModule)
		for token, provider := range importedModule.exported {
			if err := module.addVisible(token, provider); err != nil {
				return nil, err
			}
		}
	}

	for _, provider := range definition.Providers {
		state, err := newProviderState(module, provider)
		if err != nil {
			return nil, err
		}
		for _, token := range providerTokens(provider, state.resultType) {
			if err := module.addOwn(token, state); err != nil {
				return nil, err
			}
			if provider.Exported {
				module.exported[token] = state
			}
		}
	}

	return module, nil
}

type moduleContainer struct {
	name     string
	imports  []*moduleContainer
	own      map[Token]*providerState
	visible  map[Token]*providerState
	exported map[Token]*providerState
}

func (m *moduleContainer) addOwn(token Token, provider *providerState) error {
	if existing, ok := m.own[token]; ok && existing != provider {
		return duplicateProviderError(token, m.name)
	}
	m.own[token] = provider
	return m.addVisible(token, provider)
}

func (m *moduleContainer) addVisible(token Token, provider *providerState) error {
	if existing, ok := m.visible[token]; ok && existing != provider {
		return duplicateProviderError(token, m.name)
	}
	m.visible[token] = provider
	return nil
}

type providerState struct {
	module      *moduleContainer
	provider    Provider
	primary     Token
	resultType  reflect.Type
	instance    any
	initialized bool
}

func newProviderState(module *moduleContainer, provider Provider) (*providerState, error) {
	if provider.Scope != "" && provider.Scope != Singleton {
		return nil, unsupportedScopeError(provider.Scope, module.name, describeProvider(provider))
	}

	resultType, err := providerResultType(provider)
	if err != nil {
		return nil, err
	}

	state := &providerState{
		module:     module,
		provider:   provider,
		resultType: resultType,
	}
	tokens := providerTokens(provider, resultType)
	if len(tokens) > 0 {
		state.primary = tokens[0]
	}

	return state, nil
}

func (p *providerState) resolve(path []Token) (any, error) {
	if slices.Contains(path, p.primary) {
		return nil, cycleError(append(path, p.primary))
	}

	if p.initialized {
		return p.instance, nil
	}

	var (
		value any
		err   error
	)
	if p.provider.Kind == ProviderKindValue {
		value = p.provider.Value
	} else {
		value, err = invokeInModule(p.module, p.provider.Constructor, p.primary, append(path, p.primary))
		if err != nil {
			return nil, err
		}
	}

	p.instance = value
	p.initialized = true
	return value, nil
}

func invokeInModule(module *moduleContainer, constructor any, dependent Token, path ...[]Token) (any, error) {
	fn := reflect.ValueOf(constructor)
	if !fn.IsValid() || fn.Kind() != reflect.Func {
		return nil, fmt.Errorf("DI_INVALID_PROVIDER: provider %s in module %s must be a function", dependent, module.name)
	}

	fnType := fn.Type()
	args := make([]reflect.Value, 0, fnType.NumIn())
	activePath := []Token(nil)
	if len(path) > 0 {
		activePath = path[0]
	}

	for i := range fnType.NumIn() {
		token := Token{Type: fnType.In(i)}
		provider, ok := module.visible[token]
		if !ok {
			return nil, missingProviderError(token, dependent, module.name)
		}
		value, err := provider.resolve(activePath)
		if err != nil {
			return nil, err
		}
		args = append(args, reflect.ValueOf(value))
	}

	results := fn.Call(args)
	switch len(results) {
	case 1:
		return results[0].Interface(), nil
	case 2:
		if !results[1].IsNil() {
			err, ok := results[1].Interface().(error)
			if !ok {
				return nil, fmt.Errorf("DI_INVALID_PROVIDER: provider %s in module %s second return value must be an error", dependent, module.name)
			}
			return nil, err
		}
		return results[0].Interface(), nil
	default:
		return nil, fmt.Errorf("DI_INVALID_PROVIDER: provider %s in module %s must return one value or (value, error)", dependent, module.name)
	}
}

func providerResultType(provider Provider) (reflect.Type, error) {
	if provider.Kind == ProviderKindValue {
		if provider.Value == nil {
			return nil, nil
		}
		return reflect.TypeOf(provider.Value), nil
	}

	fnType := reflect.TypeOf(provider.Constructor)
	if fnType == nil || fnType.Kind() != reflect.Func {
		return nil, nil
	}
	if fnType.NumOut() == 0 {
		return nil, nil
	}
	return fnType.Out(0), nil
}

func providerTokens(provider Provider, resultType reflect.Type) []Token {
	tokens := make([]Token, 0, 2+len(provider.Aliases))
	if provider.Name != "" {
		tokens = append(tokens, Named(provider.Name))
	} else if resultType != nil {
		tokens = append(tokens, Token{Type: resultType})
	}
	tokens = append(tokens, provider.Aliases...)
	return tokens
}

func describeProvider(provider Provider) string {
	if provider.Name != "" {
		return Named(provider.Name).String()
	}
	if resultType, err := providerResultType(provider); err == nil && resultType != nil {
		return Token{Type: resultType}.String()
	}
	return "<unknown provider>"
}

type diError struct {
	Code      string
	Module    string
	Token     Token
	Dependent Token
	Scope     Scope
	Path      []Token
	Message   string
	Hint      string
}

func (e *diError) Error() string {
	parts := []string{e.Code + ": " + e.Message}
	if e.Hint != "" {
		parts = append(parts, "Hint: "+e.Hint)
	}
	return strings.Join(parts, ". ")
}

func missingProviderError(token, dependent Token, module string) error {
	message := "missing provider for " + token.String()
	if dependent != (Token{}) {
		message += " required by " + dependent.String()
	}
	message += " in module " + module

	return &diError{
		Code:      "DI_MISSING_PROVIDER",
		Module:    module,
		Token:     token,
		Dependent: dependent,
		Message:   message,
		Hint:      "add a provider or import a module that exports " + token.String(),
	}
}

func cycleError(path []Token) error {
	segments := make([]string, 0, len(path))
	for _, token := range path {
		segments = append(segments, token.String())
	}
	return &diError{
		Code:    "DI_PROVIDER_CYCLE",
		Token:   path[len(path)-1],
		Path:    append([]Token(nil), path...),
		Message: "provider cycle detected: " + strings.Join(segments, " -> "),
		Hint:    "split responsibilities or inject an interface/value that breaks the cycle",
	}
}

func duplicateProviderError(token Token, module string) error {
	return &diError{
		Code:    "DI_DUPLICATE_PROVIDER",
		Module:  module,
		Token:   token,
		Message: "duplicate provider for " + token.String() + " in module " + module,
		Hint:    "remove one provider or use a distinct name or alias",
	}
}

func unsupportedScopeError(scope Scope, module, provider string) error {
	return &diError{
		Code:    "DI_UNSUPPORTED_SCOPE",
		Module:  module,
		Scope:   scope,
		Message: "unsupported scope " + string(scope) + " for " + provider + " in module " + module,
		Hint:    "v0 supports singleton scope only",
	}
}
