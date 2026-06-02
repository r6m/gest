package validation_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r6m/gest"
	"github.com/r6m/gest/modules/validation"
)

type requestDTO struct {
	Name string `json:"name" validate:"required"`
}

type responseDTO struct {
	Name string `json:"name"`
}

type validationController struct{}

func newValidationController() *validationController {
	return &validationController{}
}

func (c *validationController) Create(ctx *gest.Context, req *requestDTO) (*responseDTO, error) {
	return &responseDTO{Name: req.Name}, nil
}

func (c *validationController) GestController() gest.ControllerDefinition {
	return gest.ControllerDefinition{
		Name: "ValidationController",
		Routes: []gest.RouteDefinition{
			{
				Method:  http.MethodPost,
				Path:    "/users",
				Handler: gest.Handle(c.Create),
			},
		},
	}
}

type validatorConsumer struct {
	validator gest.Validator
}

func newValidatorConsumer(validator gest.Validator) *validatorConsumer {
	return &validatorConsumer{validator: validator}
}

func TestNewValidatorAcceptsValidDTO(t *testing.T) {
	validator := validation.NewValidator()

	if err := validator.Validate(&requestDTO{Name: "Ada"}); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestNewValidatorRejectsRequiredTag(t *testing.T) {
	validator := validation.NewValidator()

	err := validator.Validate(&requestDTO{})
	if err == nil {
		t.Fatal("Validate returned nil error")
	}
	if !strings.Contains(err.Error(), "Name") || !strings.Contains(err.Error(), "required") {
		t.Fatalf("error = %q, want field and required tag context", err.Error())
	}
}

func TestJSONIntegrationReturns400WhenValidatorInstalled(t *testing.T) {
	app := gest.New(gest.WithValidator(validation.NewValidator()))
	app.Import(gest.NewModule(gest.ModuleConfig{
		Name:      "AppModule",
		Providers: gest.Providers(gest.Controller(newValidationController)),
	}))

	recorder := httptest.NewRecorder()
	app.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{}`)))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	var body struct {
		Error gest.HTTPError `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("Unmarshal response returned error: %v", err)
	}
	if body.Error.Code != "BINDING_VALIDATION_FAILURE" {
		t.Fatalf("error code = %q, want BINDING_VALIDATION_FAILURE", body.Error.Code)
	}
	if !strings.Contains(body.Error.Message, "required") {
		t.Fatalf("error message = %q, want required validation context", body.Error.Message)
	}
}

func TestValidationModuleProvidesValidatorThroughDI(t *testing.T) {
	mod := gest.NewModule(gest.ModuleConfig{
		Name: "AppModule",
		Imports: gest.Imports(
			validation.Module(validation.Options{}),
		),
		Providers: gest.Providers(
			gest.Provide(newValidatorConsumer),
		),
	})
	container := newContainer(t, mod)

	value, err := container.Resolve(gest.TokenOf[*validatorConsumer]())
	if err != nil {
		t.Fatalf("Resolve *validatorConsumer returned error: %v", err)
	}
	consumer, ok := value.(*validatorConsumer)
	if !ok {
		t.Fatalf("resolved value = %T, want *validatorConsumer", value)
	}
	if err := consumer.validator.Validate(&requestDTO{Name: "Ada"}); err != nil {
		t.Fatalf("injected validator returned error: %v", err)
	}

	resolved, err := container.Resolve(gest.TokenOf[gest.Validator]())
	if err != nil {
		t.Fatalf("Resolve gest.Validator returned error: %v", err)
	}
	if _, ok := resolved.(gest.Validator); !ok {
		t.Fatalf("resolved value = %T, want gest.Validator", resolved)
	}
}

func TestValidationModuleIsOptional(t *testing.T) {
	container := newContainer(t, gest.NewModule(gest.ModuleConfig{Name: "App"}))

	_, err := container.Resolve(gest.TokenOf[gest.Validator]())
	if err == nil {
		t.Fatal("Resolve gest.Validator returned nil error without importing validation module")
	}
}

func TestCoreRuntimeDoesNotImportValidationModule(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	matches, err := filepath.Glob(filepath.Join(root, "*.go"))
	if err != nil {
		t.Fatalf("Glob returned error: %v", err)
	}
	for _, file := range matches {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("ReadFile %s returned error: %v", file, err)
		}
		if strings.Contains(string(content), "github.com/r6m/gest/modules/validation") {
			t.Fatalf("core runtime file %s imports modules/validation", file)
		}
	}
}

func TestValidationErrorCodeStableThroughContext(t *testing.T) {
	ctx := gest.NewContext(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", nil))
	ctx.SetValidator(validation.NewValidator())

	err := ctx.Validate(&requestDTO{})
	if err == nil {
		t.Fatal("Validate returned nil error")
	}
	var httpErr *gest.HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("error type = %T, want *gest.HTTPError", err)
	}
	if httpErr.Code != "BINDING_VALIDATION_FAILURE" {
		t.Fatalf("error code = %q, want BINDING_VALIDATION_FAILURE", httpErr.Code)
	}
}

func newContainer(t *testing.T, mod gest.Module) gest.Container {
	t.Helper()
	container, err := gest.NewContainer(mod)
	if err != nil {
		t.Fatalf("NewContainer returned error: %v", err)
	}
	return container
}
