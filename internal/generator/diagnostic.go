package generator

import (
	"fmt"
	"slices"
)

const (
	SeverityError = "error"

	DiagnosticUnknownDecorator        = "GEN_UNKNOWN_DECORATOR"
	DiagnosticInvalidDecoratorSyntax  = "GEN_INVALID_DECORATOR_SYNTAX"
	DiagnosticInvalidTarget           = "GEN_INVALID_TARGET"
	DiagnosticInvalidHandlerSignature = "HANDLER_INVALID_SIGNATURE"
	DiagnosticFormatFailure           = "GEN_FORMAT_FAILURE"
	DiagnosticWriteFailure            = "GEN_WRITE_FAILURE"
)

// Diagnostic describes a generator problem with source location when available.
type Diagnostic struct {
	Severity string
	Code     string
	Message  string
	Hint     string
	File     string
	Line     int
	Column   int
	Target   string
}

func (d Diagnostic) Error() string {
	if d.File != "" && d.Line > 0 {
		return fmt.Sprintf("%s:%d: %s: %s", d.File, d.Line, d.Code, d.Message)
	}
	return d.Code + ": " + d.Message
}

func sortDiagnostics(diagnostics []Diagnostic) {
	slices.SortFunc(diagnostics, func(a Diagnostic, b Diagnostic) int {
		if a.File < b.File {
			return -1
		}
		if a.File > b.File {
			return 1
		}
		if a.Line < b.Line {
			return -1
		}
		if a.Line > b.Line {
			return 1
		}
		if a.Code < b.Code {
			return -1
		}
		if a.Code > b.Code {
			return 1
		}
		return 0
	})
}
