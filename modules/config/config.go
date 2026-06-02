package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/r6m/gest"
)

// Options configures the optional runtime config module.
type Options struct {
	EnvFiles      []string
	RequiredFiles []string
	Load          []LoadTarget
}

// LoadTarget declares an app-owned config value to load and provide through DI.
type LoadTarget interface {
	provider() gest.Provider
}

// Struct declares a user-owned struct pointer provider loaded from environment.
func Struct[T any]() LoadTarget {
	return structTarget[T]{}
}

// Service stores merged runtime configuration values.
type Service struct {
	values map[string]string
}

// Module returns a Gest module that provides runtime config values.
func Module(options Options) gest.Module {
	providers := []gest.Provider{
		gest.Provide(func() (*Service, error) {
			return NewService(options)
		}),
	}
	for _, target := range options.Load {
		if target != nil {
			providers = append(providers, target.provider())
		}
	}

	return gest.NewModule(gest.ModuleConfig{
		Name:      "ConfigModule",
		Providers: gest.Providers(providers...),
	})
}

// NewService loads configuration values according to options.
func NewService(options Options) (*Service, error) {
	values := make(map[string]string)
	for _, file := range options.EnvFiles {
		if err := loadFile(values, file, false); err != nil {
			return nil, err
		}
	}
	for _, file := range options.RequiredFiles {
		if err := loadFile(values, file, true); err != nil {
			return nil, err
		}
	}
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[key] = value
		}
	}
	return &Service{values: values}, nil
}

// Get returns a config value or an empty string.
func (s *Service) Get(key string) string {
	if s == nil {
		return ""
	}
	return s.values[key]
}

// Required returns a config value or a structured missing-key error.
func (s *Service) Required(key string) (string, error) {
	value := s.Get(key)
	if value == "" {
		return "", &Error{Code: "CONFIG_REQUIRED_KEY_MISSING", Key: key, Message: "required config key is missing or empty"}
	}
	return value, nil
}

// Int returns an int config value.
func (s *Service) Int(key string) (int, error) {
	value := s.Get(key)
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, conversionError(key, "", "int", value, err)
	}
	return parsed, nil
}

// Bool returns a bool config value.
func (s *Service) Bool(key string) (bool, error) {
	value := s.Get(key)
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, conversionError(key, "", "bool", value, err)
	}
	return parsed, nil
}

// Float returns a float64 config value.
func (s *Service) Float(key string) (float64, error) {
	value := s.Get(key)
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, conversionError(key, "", "float64", value, err)
	}
	return parsed, nil
}

type structTarget[T any] struct{}

func (t structTarget[T]) provider() gest.Provider {
	return gest.Provide(func(service *Service) (*T, error) {
		var value T
		if err := service.Load(&value); err != nil {
			return nil, err
		}
		return &value, nil
	})
}

// Load fills target from env tags, default tags, and validate tags.
func (s *Service) Load(target any) error {
	if target == nil {
		return &Error{Code: "CONFIG_INVALID_TARGET", Message: "config load target must not be nil"}
	}
	value := reflect.ValueOf(target)
	if value.Kind() != reflect.Pointer || value.IsNil() || value.Elem().Kind() != reflect.Struct {
		return &Error{Code: "CONFIG_INVALID_TARGET", Message: "config load target must be a pointer to a struct"}
	}
	return s.loadStruct(value.Elem())
}

func (s *Service) loadStruct(value reflect.Value) error {
	valueType := value.Type()
	for i := range value.NumField() {
		field := valueType.Field(i)
		fieldValue := value.Field(i)
		if !fieldValue.CanSet() {
			continue
		}
		key := field.Tag.Get("env")
		if key == "" {
			key = field.Name
		}
		raw := s.Get(key)
		if raw == "" {
			raw = field.Tag.Get("default")
		}
		if raw == "" && field.Tag.Get("validate") == "required" {
			return &Error{
				Code:    "CONFIG_REQUIRED_FIELD_MISSING",
				Key:     key,
				Field:   field.Name,
				Message: "required config field is missing or empty",
			}
		}
		if raw == "" {
			continue
		}
		if err := setField(fieldValue, raw); err != nil {
			return conversionError(key, field.Name, fieldValue.Type().String(), raw, err)
		}
	}
	return nil
}

func setField(field reflect.Value, raw string) error {
	if field.Type() == reflect.TypeFor[time.Duration]() {
		duration, err := time.ParseDuration(raw)
		if err != nil {
			return err
		}
		field.SetInt(int64(duration))
		return nil
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(raw)
	case reflect.Bool:
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return err
		}
		field.SetBool(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value, err := strconv.ParseInt(raw, 10, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetInt(value)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		value, err := strconv.ParseUint(raw, 10, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetUint(value)
	case reflect.Float32, reflect.Float64:
		value, err := strconv.ParseFloat(raw, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetFloat(value)
	default:
		return fmt.Errorf("unsupported field type %s", field.Type())
	}
	return nil
}

func loadFile(values map[string]string, file string, required bool) error {
	handle, err := os.Open(file)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && !required {
			return nil
		}
		code := "CONFIG_ENV_FILE_OPEN_FAILED"
		if errors.Is(err, os.ErrNotExist) {
			code = "CONFIG_REQUIRED_FILE_MISSING"
		}
		return &Error{Code: code, File: file, Message: "could not open env file", Err: err}
	}
	defer func() {
		_ = handle.Close()
	}()

	scanner := bufio.NewScanner(handle)
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		key, value, ok := strings.Cut(text, "=")
		if !ok {
			return &Error{Code: "CONFIG_ENV_FILE_PARSE_FAILED", File: file, Line: line, Message: "expected KEY=value"}
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			return &Error{Code: "CONFIG_ENV_FILE_PARSE_FAILED", File: file, Line: line, Message: "env key must not be empty"}
		}
		values[key] = unquote(value)
	}
	if err := scanner.Err(); err != nil {
		return &Error{Code: "CONFIG_ENV_FILE_READ_FAILED", File: file, Message: "could not read env file", Err: err}
	}
	return nil
}

func unquote(value string) string {
	if len(value) < 2 {
		return value
	}
	unquoted, err := strconv.Unquote(value)
	if err != nil {
		return value
	}
	return unquoted
}

func conversionError(key, field, targetType, value string, err error) error {
	return &Error{
		Code:       "CONFIG_CONVERSION_FAILED",
		Key:        key,
		Field:      field,
		TargetType: targetType,
		Value:      value,
		Message:    "could not convert config value",
		Err:        err,
	}
}

// Error is a structured config module error.
type Error struct {
	Code       string
	File       string
	Line       int
	Key        string
	Field      string
	TargetType string
	Value      string
	Message    string
	Err        error
}

func (e *Error) Error() string {
	parts := []string{e.Code + ": " + e.Message}
	if e.File != "" {
		location := e.File
		if e.Line > 0 {
			location += ":" + strconv.Itoa(e.Line)
		}
		parts = append(parts, "file "+location)
	}
	if e.Field != "" {
		parts = append(parts, "field "+e.Field)
	}
	if e.Key != "" {
		parts = append(parts, "key "+e.Key)
	}
	if e.TargetType != "" {
		parts = append(parts, "target "+e.TargetType)
	}
	if e.Value != "" {
		parts = append(parts, "value "+strconv.Quote(e.Value))
	}
	if e.Err != nil {
		parts = append(parts, "cause "+e.Err.Error())
	}
	return strings.Join(parts, ". ")
}

func (e *Error) Unwrap() error {
	return e.Err
}
