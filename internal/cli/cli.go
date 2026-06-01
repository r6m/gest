package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Handler runs a parsed CLI command.
type Handler func(context.Context) error

// CLI contains the command handlers used by the gest executable.
type CLI struct {
	Generate           Handler
	Build              Handler
	GenerateModule     Handler
	GenerateController Handler
	GenerateService    Handler
}

// New creates a CLI with placeholder handlers for commands implemented in later phases.
func New() *CLI {
	return &CLI{
		Generate:           unimplemented("generate"),
		Build:              unimplemented("build"),
		GenerateModule:     unimplemented("g module"),
		GenerateController: unimplemented("g controller"),
		GenerateService:    unimplemented("g service"),
	}
}

// Run parses args, writes command output, and returns the process exit code.
func (c *CLI) Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if c == nil {
		c = New()
	}

	if len(args) == 0 || isHelp(args[0]) {
		if err := writeHelp(stdout); err != nil {
			return 1
		}
		return 0
	}

	var err error
	switch args[0] {
	case "help":
		if err := writeHelp(stdout); err != nil {
			return 1
		}
		return 0
	case "generate":
		err = runHandler(ctx, c.Generate)
	case "build":
		err = runHandler(ctx, c.Build)
	case "g":
		err = c.runGenerateShortcut(ctx, args[1:])
	default:
		err = fmt.Errorf("unknown command %q", args[0])
	}

	if err != nil {
		if _, writeErr := fmt.Fprintf(stderr, "error: %v\n", err); writeErr != nil {
			return 1
		}
		return 1
	}

	return 0
}

func (c *CLI) runGenerateShortcut(ctx context.Context, args []string) error {
	if len(args) == 0 || isHelp(args[0]) {
		return errors.New("g requires a subcommand: module, controller, or service")
	}

	switch args[0] {
	case "module":
		return runHandler(ctx, c.GenerateModule)
	case "controller":
		return runHandler(ctx, c.GenerateController)
	case "service":
		return runHandler(ctx, c.GenerateService)
	default:
		return fmt.Errorf("unknown g subcommand %q", args[0])
	}
}

func runHandler(ctx context.Context, handler Handler) error {
	if handler == nil {
		return errors.New("command handler is not configured")
	}

	return handler(ctx)
}

func unimplemented(name string) Handler {
	return func(context.Context) error {
		return fmt.Errorf("%s is not implemented yet", name)
	}
}

func isHelp(arg string) bool {
	return arg == "-h" || arg == "--help"
}

func writeHelp(w io.Writer) error {
	_, err := fmt.Fprint(w, strings.TrimLeft(helpText, "\n"))
	return err
}

const helpText = `
gest is the command line tool for Gest projects.

Usage:
  gest <command>
  gest g <subcommand>

Commands:
  gest generate      generate Gest metadata
  gest build         generate and build the project
  gest g module      scaffold a module
  gest g controller  scaffold a controller
  gest g service     scaffold a service
  gest help          show this help
`
