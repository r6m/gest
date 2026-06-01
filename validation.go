package gest

// Validator validates a bound request DTO.
type Validator interface {
	Validate(any) error
}

type noopValidator struct{}

func (noopValidator) Validate(any) error {
	return nil
}
