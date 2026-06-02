package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/r6m/gest"
)

const (
	defaultLevel  = "info"
	defaultFormat = "text"
)

// Options configures the optional logger module.
type Options struct {
	Level  string
	Format string
	Writer io.Writer
}

// Module returns a Gest module that provides *slog.Logger through DI.
func Module(options Options) gest.Module {
	return gest.NewModule(gest.ModuleConfig{
		Name:   "LoggerModule",
		Global: true,
		Providers: gest.Providers(
			gest.Provide(func() (*slog.Logger, error) {
				return NewLogger(options)
			}),
		),
	})
}

// NewLogger creates a configured slog logger.
func NewLogger(options Options) (*slog.Logger, error) {
	level, err := parseLevel(options.Level)
	if err != nil {
		return nil, err
	}
	format, err := parseFormat(options.Format)
	if err != nil {
		return nil, err
	}
	writer := options.Writer
	if writer == nil {
		writer = os.Stdout
	}

	handlerOptions := &slog.HandlerOptions{Level: level}
	switch format {
	case defaultFormat:
		return slog.New(slog.NewTextHandler(writer, handlerOptions)), nil
	case "json":
		return slog.New(slog.NewJSONHandler(writer, handlerOptions)), nil
	default:
		return nil, &Error{Code: "LOGGER_INVALID_FORMAT", Value: format, Message: "unsupported logger format"}
	}
}

func parseLevel(value string) (slog.Level, error) {
	if value == "" {
		value = defaultLevel
	}
	switch strings.ToLower(value) {
	case "debug":
		return slog.LevelDebug, nil
	case defaultLevel:
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, &Error{
			Code:    "LOGGER_INVALID_LEVEL",
			Field:   "Level",
			Value:   value,
			Message: "unsupported logger level; expected debug, info, warn, or error",
		}
	}
}

func parseFormat(value string) (string, error) {
	if value == "" {
		value = defaultFormat
	}
	switch strings.ToLower(value) {
	case defaultFormat:
		return defaultFormat, nil
	case "json":
		return "json", nil
	default:
		return "", &Error{
			Code:    "LOGGER_INVALID_FORMAT",
			Field:   "Format",
			Value:   value,
			Message: "unsupported logger format; expected text or json",
		}
	}
}

// Error is a structured logger module error.
type Error struct {
	Code    string
	Field   string
	Value   string
	Message string
}

func (e *Error) Error() string {
	message := e.Code + ": " + e.Message
	if e.Field != "" {
		message += ". field " + e.Field
	}
	if e.Value != "" {
		message += ". value " + fmt.Sprintf("%q", e.Value)
	}
	return message
}
