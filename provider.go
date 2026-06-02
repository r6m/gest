package gest

// ProviderKind identifies the kind of provider declaration.
type ProviderKind string

const (
	ProviderKindService    ProviderKind = "service"
	ProviderKindController ProviderKind = "controller"
	ProviderKindValue      ProviderKind = "value"
)

// Scope identifies a provider lifecycle.
type Scope string

const (
	Singleton Scope = "singleton"

	// Transient is deferred in v0. Implementations must reject it with a clear error.
	Transient Scope = "transient"

	// Request is deferred in v0. Implementations must reject it with a clear error.
	Request Scope = "request"
)

// Provider declares a runtime provider. It does not resolve dependencies.
type Provider struct {
	Kind ProviderKind

	Constructor any
	Value       any

	Scope Scope

	Name    string
	Aliases []Token
}

// ProviderOption configures a provider declaration.
type ProviderOption func(*Provider)

// Provide declares a singleton service provider.
func Provide(constructor any, options ...ProviderOption) Provider {
	p := Provider{
		Kind:        ProviderKindService,
		Constructor: constructor,
		Scope:       Singleton,
	}

	for _, option := range options {
		option(&p)
	}

	return p
}

// Controller declares a controller provider.
func Controller(constructor any, options ...ProviderOption) Provider {
	p := Provide(constructor, options...)
	p.Kind = ProviderKindController
	return p
}

// Value declares a singleton value provider.
func Value(value any, options ...ProviderOption) Provider {
	p := Provider{
		Kind:  ProviderKindValue,
		Value: value,
		Scope: Singleton,
	}

	for _, option := range options {
		option(&p)
	}

	return p
}

// Name assigns a string name to a provider.
func Name(name string) ProviderOption {
	return func(p *Provider) {
		p.Name = name
	}
}

// As adds a token alias for T.
func As[T any]() ProviderOption {
	return func(p *Provider) {
		p.Aliases = append(p.Aliases, TokenOf[T]())
	}
}

// WithScope records a provider scope. Non-singleton behavior is not implemented in v0.
func WithScope(scope Scope) ProviderOption {
	return func(p *Provider) {
		p.Scope = scope
	}
}
