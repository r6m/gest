package jwt

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	golangjwt "github.com/golang-jwt/jwt/v5"

	"github.com/r6m/gest"
)

const defaultAccessTTL = time.Hour

// Options configures the optional JWT module.
type Options struct {
	Secret        string
	SecretFromEnv string
	Issuer        string
	AccessTTL     time.Duration
}

// Claims contains verified JWT claims.
type Claims struct {
	Subject   string
	Issuer    string
	ExpiresAt time.Time
	IssuedAt  time.Time
	Values    map[string]any
}

// ClaimOption adds custom claims to a signed token.
type ClaimOption func(map[string]any)

// Value adds a custom claim value.
func Value(key string, value any) ClaimOption {
	return func(values map[string]any) {
		values[key] = value
	}
}

// Service signs and verifies JWT access tokens.
type Service struct {
	secret    []byte
	issuer    string
	accessTTL time.Duration
}

// Module returns a Gest module that provides *jwt.Service through DI.
func Module(options Options) gest.Module {
	return gest.NewModule(gest.ModuleConfig{
		Name: "JWTModule",
		Providers: gest.Providers(
			gest.Provide(func() (*Service, error) {
				return NewService(options)
			}),
		),
	})
}

// NewService creates a JWT service from explicit options.
func NewService(options Options) (*Service, error) {
	secret := options.Secret
	if secret == "" && options.SecretFromEnv != "" {
		secret = os.Getenv(options.SecretFromEnv)
	}
	if secret == "" {
		return nil, &Error{
			Code:    "JWT_SECRET_MISSING",
			Field:   "Secret",
			Message: "JWT secret is required; set Options.Secret or Options.SecretFromEnv",
		}
	}
	ttl := options.AccessTTL
	if ttl == 0 {
		ttl = defaultAccessTTL
	}
	if ttl < 0 {
		return nil, &Error{
			Code:    "JWT_INVALID_ACCESS_TTL",
			Field:   "AccessTTL",
			Value:   ttl.String(),
			Message: "JWT access TTL must be positive",
		}
	}
	return &Service{
		secret:    []byte(secret),
		issuer:    options.Issuer,
		accessTTL: ttl,
	}, nil
}

// Sign creates a signed access token for a subject.
func (s *Service) Sign(subject string, values ...ClaimOption) (string, error) {
	if subject == "" {
		return "", &Error{Code: "JWT_SUBJECT_MISSING", Field: "subject", Message: "JWT subject is required"}
	}
	now := time.Now()
	claims := golangjwt.MapClaims{
		"sub": subject,
		"iat": golangjwt.NewNumericDate(now),
		"exp": golangjwt.NewNumericDate(now.Add(s.accessTTL)),
	}
	if s.issuer != "" {
		claims["iss"] = s.issuer
	}
	for _, option := range values {
		if option != nil {
			option(claims)
		}
	}

	token, err := golangjwt.NewWithClaims(golangjwt.SigningMethodHS256, claims).SignedString(s.secret)
	if err != nil {
		return "", &Error{Code: "JWT_SIGN_FAILED", Message: "could not sign JWT", Err: err}
	}
	return token, nil
}

// Verify verifies a token signature and registered claims.
func (s *Service) Verify(token string) (*Claims, error) {
	if strings.TrimSpace(token) == "" {
		return nil, &Error{Code: "JWT_TOKEN_MISSING", Message: "JWT token is required"}
	}

	claims := golangjwt.MapClaims{}
	parsed, err := golangjwt.ParseWithClaims(token, claims, func(parsed *golangjwt.Token) (any, error) {
		if parsed.Method != golangjwt.SigningMethodHS256 {
			return nil, &Error{Code: "JWT_INVALID_SIGNING_METHOD", Message: "unexpected JWT signing method"}
		}
		return s.secret, nil
	}, golangjwt.WithIssuer(s.issuer), golangjwt.WithExpirationRequired())
	if err != nil {
		return nil, verifyError(err)
	}
	if parsed == nil || !parsed.Valid {
		return nil, &Error{Code: "JWT_INVALID_TOKEN", Message: "JWT token is invalid"}
	}

	return parseClaims(claims)
}

func parseClaims(claims golangjwt.MapClaims) (*Claims, error) {
	subject, err := claims.GetSubject()
	if err != nil {
		return nil, &Error{Code: "JWT_INVALID_CLAIMS", Field: "sub", Message: "JWT subject claim is invalid", Err: err}
	}
	issuer, err := claims.GetIssuer()
	if err != nil {
		return nil, &Error{Code: "JWT_INVALID_CLAIMS", Field: "iss", Message: "JWT issuer claim is invalid", Err: err}
	}
	expiresAt, err := claims.GetExpirationTime()
	if err != nil {
		return nil, &Error{Code: "JWT_INVALID_CLAIMS", Field: "exp", Message: "JWT expiration claim is invalid", Err: err}
	}
	issuedAt, err := claims.GetIssuedAt()
	if err != nil {
		return nil, &Error{Code: "JWT_INVALID_CLAIMS", Field: "iat", Message: "JWT issued-at claim is invalid", Err: err}
	}

	values := make(map[string]any)
	for key, value := range claims {
		switch key {
		case "sub", "iss", "exp", "iat", "nbf", "aud", "jti":
			continue
		default:
			values[key] = value
		}
	}

	result := &Claims{
		Subject: subject,
		Issuer:  issuer,
		Values:  values,
	}
	if expiresAt != nil {
		result.ExpiresAt = expiresAt.Time
	}
	if issuedAt != nil {
		result.IssuedAt = issuedAt.Time
	}
	return result, nil
}

func verifyError(err error) error {
	switch {
	case errors.Is(err, golangjwt.ErrTokenExpired):
		return &Error{Code: "JWT_TOKEN_EXPIRED", Message: "JWT token is expired", Err: err}
	case errors.Is(err, golangjwt.ErrTokenInvalidIssuer):
		return &Error{Code: "JWT_ISSUER_MISMATCH", Field: "iss", Message: "JWT issuer does not match configured issuer", Err: err}
	case errors.Is(err, golangjwt.ErrTokenSignatureInvalid):
		return &Error{Code: "JWT_INVALID_SIGNATURE", Message: "JWT signature is invalid", Err: err}
	default:
		return &Error{Code: "JWT_VERIFY_FAILED", Message: "could not verify JWT", Err: err}
	}
}

// Error is a structured JWT module error.
type Error struct {
	Code    string
	Field   string
	Value   string
	Message string
	Err     error
}

func (e *Error) Error() string {
	parts := []string{e.Code + ": " + e.Message}
	if e.Field != "" {
		parts = append(parts, "field "+e.Field)
	}
	if e.Value != "" {
		parts = append(parts, "value "+fmt.Sprintf("%q", e.Value))
	}
	if e.Err != nil {
		parts = append(parts, "cause "+e.Err.Error())
	}
	return strings.Join(parts, ". ")
}

func (e *Error) Unwrap() error {
	return e.Err
}
