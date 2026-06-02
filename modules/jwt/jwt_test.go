package jwt_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/r6m/gest"
	"github.com/r6m/gest/modules/jwt"
)

func TestServiceSignsAndVerifiesToken(t *testing.T) {
	service := newService(t, jwt.Options{
		Secret:    "secret",
		Issuer:    "my-api",
		AccessTTL: time.Hour,
	})

	token, err := service.Sign("user-1", jwt.Value("role", "admin"))
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}
	claims, err := service.Verify(token)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}

	if claims.Subject != "user-1" {
		t.Fatalf("Subject = %q, want user-1", claims.Subject)
	}
	if claims.Issuer != "my-api" {
		t.Fatalf("Issuer = %q, want my-api", claims.Issuer)
	}
	if claims.ExpiresAt.IsZero() {
		t.Fatal("ExpiresAt is zero")
	}
	if claims.IssuedAt.IsZero() {
		t.Fatal("IssuedAt is zero")
	}
	if claims.Values["role"] != "admin" {
		t.Fatalf("role = %#v, want admin", claims.Values["role"])
	}
}

func TestInvalidSignatureFails(t *testing.T) {
	signer := newService(t, jwt.Options{Secret: "signing-secret"})
	verifier := newService(t, jwt.Options{Secret: "other-secret"})
	token, err := signer.Sign("user-1")
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}

	_, err = verifier.Verify(token)
	assertJWTError(t, err, "JWT_INVALID_SIGNATURE")
}

func TestExpiredTokenFails(t *testing.T) {
	service := newService(t, jwt.Options{
		Secret:    "secret",
		AccessTTL: time.Nanosecond,
	})
	token, err := service.Sign("user-1")
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}
	time.Sleep(2 * time.Millisecond)

	_, err = service.Verify(token)
	assertJWTError(t, err, "JWT_TOKEN_EXPIRED")
}

func TestIssuerMismatchFails(t *testing.T) {
	signer := newService(t, jwt.Options{Secret: "secret", Issuer: "issuer-a"})
	verifier := newService(t, jwt.Options{Secret: "secret", Issuer: "issuer-b"})
	token, err := signer.Sign("user-1")
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}

	_, err = verifier.Verify(token)
	assertJWTError(t, err, "JWT_ISSUER_MISMATCH")
}

func TestSecretWinsOverSecretFromEnv(t *testing.T) {
	t.Setenv("JWT_SECRET", "env-secret")
	service := newService(t, jwt.Options{
		Secret:        "option-secret",
		SecretFromEnv: "JWT_SECRET",
	})
	envService := newService(t, jwt.Options{Secret: "env-secret"})

	token, err := service.Sign("user-1")
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}

	if _, err := envService.Verify(token); err == nil {
		t.Fatal("env service verified token, want option Secret to win")
	}
	if _, err := service.Verify(token); err != nil {
		t.Fatalf("service Verify returned error: %v", err)
	}
}

func TestSecretFromEnvWorks(t *testing.T) {
	t.Setenv("JWT_SECRET", "env-secret")
	service := newService(t, jwt.Options{SecretFromEnv: "JWT_SECRET"})

	token, err := service.Sign("user-1")
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}
	if _, err := service.Verify(token); err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
}

func TestMissingSecretFailsAtStartup(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	container := newContainer(t, jwt.Module(jwt.Options{SecretFromEnv: "JWT_SECRET"}))

	_, err := container.Resolve(gest.TokenOf[*jwt.Service]())
	assertJWTError(t, err, "JWT_SECRET_MISSING")
}

func TestServiceResolvesThroughGestDI(t *testing.T) {
	container := newContainer(t, jwt.Module(jwt.Options{Secret: "secret"}))

	value, err := container.Resolve(gest.TokenOf[*jwt.Service]())
	if err != nil {
		t.Fatalf("Resolve *jwt.Service returned error: %v", err)
	}
	service, ok := value.(*jwt.Service)
	if !ok {
		t.Fatalf("resolved value = %T, want *jwt.Service", value)
	}
	token, err := service.Sign("user-1")
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}
	if _, err := service.Verify(token); err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
}

func TestJWTModuleIsOptional(t *testing.T) {
	container := newContainer(t, gest.NewModule(gest.ModuleConfig{Name: "App"}))

	_, err := container.Resolve(gest.TokenOf[*jwt.Service]())
	if err == nil {
		t.Fatal("Resolve *jwt.Service returned nil error without importing JWT module")
	}
}

func TestCoreRuntimeDoesNotImportJWTModule(t *testing.T) {
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
		if strings.Contains(string(content), "github.com/r6m/gest/modules/jwt") {
			t.Fatalf("core runtime file %s imports modules/jwt", file)
		}
	}
}

func newService(t *testing.T, options jwt.Options) *jwt.Service {
	t.Helper()
	service, err := jwt.NewService(options)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}
	return service
}

func newContainer(t *testing.T, mod gest.Module) gest.Container {
	t.Helper()
	container, err := gest.NewContainer(mod)
	if err != nil {
		t.Fatalf("NewContainer returned error: %v", err)
	}
	return container
}

func assertJWTError(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want %s", code)
	}
	var jwtErr *jwt.Error
	if !errors.As(err, &jwtErr) {
		t.Fatalf("error type = %T, want *jwt.Error; error %v", err, err)
	}
	if jwtErr.Code != code {
		t.Fatalf("error code = %q, want %q; error %v", jwtErr.Code, code, err)
	}
}
