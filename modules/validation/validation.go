package validation

import (
	playground "github.com/go-playground/validator/v10"

	"github.com/r6m/gest"
)

// Options configures the optional validation module.
type Options struct{}

// Module returns a Gest module that provides a concrete validator through DI.
func Module(options Options) gest.Module {
	return gest.NewModule(gest.ModuleConfig{
		Name: "ValidationModule",
		Providers: gest.Providers(
			gest.Value(NewValidator(), gest.As[gest.Validator]()),
		),
	})
}

// NewValidator returns a validator compatible with gest.WithValidator.
func NewValidator() gest.Validator {
	return &Validator{
		validate: playground.New(playground.WithRequiredStructEnabled()),
	}
}

// Validator validates DTOs using validate tags.
type Validator struct {
	validate *playground.Validate
}

// Validate validates a value using go-playground/validator tags.
func (v *Validator) Validate(value any) error {
	return v.validate.Struct(value)
}
