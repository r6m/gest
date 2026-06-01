package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// Handler runs a parsed CLI command.
type Handler func(context.Context, []string) error

// CLI contains the command handlers used by the gest executable.
type CLI struct {
	Generate           Handler
	Build              Handler
	GenerateModule     Handler
	GenerateController Handler
	GenerateService    Handler
	WorkDir            string
	Stdout             io.Writer
}

// New creates a CLI with placeholder handlers for commands implemented in later phases.
func New() *CLI {
	command := &CLI{}
	command.Generate = command.runGenerate
	command.Build = unimplemented("build")
	command.GenerateModule = unimplemented("g module")
	command.GenerateController = unimplemented("g controller")
	command.GenerateService = unimplemented("g service")
	return command
}

func (c *CLI) withDefaults(stdout io.Writer) *CLI {
	if c == nil {
		return New().withDefaults(stdout)
	}
	if c.Generate == nil {
		c.Generate = c.runGenerate
	}
	if c.Build == nil {
		c.Build = unimplemented("build")
	}
	if c.GenerateModule == nil {
		c.GenerateModule = unimplemented("g module")
	}
	if c.GenerateController == nil {
		c.GenerateController = unimplemented("g controller")
	}
	if c.GenerateService == nil {
		c.GenerateService = unimplemented("g service")
	}
	if c.WorkDir == "" {
		if workDir, err := os.Getwd(); err == nil {
			c.WorkDir = workDir
		}
	}
	c.Stdout = stdout
	return c
}

// Run parses args, writes command output, and returns the process exit code.
func (c *CLI) Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	c = c.withDefaults(stdout)

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
		err = runHandler(ctx, c.Generate, args[1:])
	case "build":
		err = runHandler(ctx, c.Build, args[1:])
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
		return runHandler(ctx, c.GenerateModule, args[1:])
	case "controller":
		return runHandler(ctx, c.GenerateController, args[1:])
	case "service":
		return runHandler(ctx, c.GenerateService, args[1:])
	default:
		return fmt.Errorf("unknown g subcommand %q", args[0])
	}
}

func runHandler(ctx context.Context, handler Handler, args []string) error {
	if handler == nil {
		return errors.New("command handler is not configured")
	}

	return handler(ctx, args)
}

func unimplemented(name string) Handler {
	return func(context.Context, []string) error {
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
