package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/r6m/gest/internal/config"
)

func (c *CLI) runBuild(ctx context.Context, args []string) error {
	options, err := parseBuildOptions(args)
	if err != nil {
		return err
	}

	cfg, err := config.Load(c.WorkDir)
	if err != nil {
		return err
	}

	buildCfg := resolveBuildConfig(cfg, options)
	if buildCfg.generate {
		root := cfg.Generate.Root
		if !filepath.IsAbs(root) {
			root = filepath.Join(c.WorkDir, root)
		}
		result, err := runGenerate(filepath.Clean(root), false)
		if err != nil {
			return err
		}
		if err := writeGenerateOutput(c.Stdout, result); err != nil {
			return err
		}
		if hasErrorDiagnostics(result.diagnostics) {
			return fmt.Errorf("generation failed")
		}
	}

	if buildCfg.test {
		if err := c.runGoCommand(ctx, []string{"test", "./..."}); err != nil {
			return err
		}
	}

	buildArgs := []string{"build"}
	if cfg.Build.Trimpath {
		buildArgs = append(buildArgs, "-trimpath")
	}
	if options.race {
		buildArgs = append(buildArgs, "-race")
	}
	if options.tags != "" {
		buildArgs = append(buildArgs, "-tags", options.tags)
	}
	if options.ldflags != "" {
		buildArgs = append(buildArgs, "-ldflags", options.ldflags)
	}
	buildArgs = append(buildArgs, "-o", buildCfg.output, buildCfg.entry)
	if err := ensureOutputDir(c.WorkDir, buildCfg.output); err != nil {
		return err
	}
	return c.runGoCommand(ctx, buildArgs)
}

type buildOptions struct {
	entry    string
	output   string
	generate *bool
	test     *bool
	race     bool
	tags     string
	ldflags  string
}

func parseBuildOptions(args []string) (buildOptions, error) {
	var options buildOptions
	flags := flag.NewFlagSet("build", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&options.entry, "entry", "", "build entry package")
	flags.StringVar(&options.output, "out", "", "build output path")
	flags.BoolVar(&options.race, "race", false, "enable race detector")
	flags.StringVar(&options.tags, "tags", "", "go build tags")
	flags.StringVar(&options.ldflags, "ldflags", "", "go linker flags")
	flags.BoolFunc("no-generate", "skip generation before building", func(valueText string) error {
		value := !parseFlagBool(valueText)
		options.generate = &value
		return nil
	})
	flags.BoolFunc("test", "run go test ./... before building", func(valueText string) error {
		value := parseFlagBool(valueText)
		options.test = &value
		return nil
	})
	flags.BoolFunc("no-test", "skip go test ./... before building", func(valueText string) error {
		value := !parseFlagBool(valueText)
		options.test = &value
		return nil
	})

	if err := flags.Parse(args); err != nil {
		return buildOptions{}, fmt.Errorf("parse build flags: %w", err)
	}
	if flags.NArg() > 0 {
		return buildOptions{}, fmt.Errorf("unexpected build argument %q", flags.Arg(0))
	}
	return options, nil
}

func parseFlagBool(value string) bool {
	return value == "true"
}

type resolvedBuildConfig struct {
	entry    string
	output   string
	generate bool
	test     bool
}

func resolveBuildConfig(cfg config.Config, options buildOptions) resolvedBuildConfig {
	resolved := resolvedBuildConfig{
		entry:    cfg.Build.Entry,
		output:   cfg.Build.Output,
		generate: cfg.Build.Generate,
		test:     cfg.Build.Test,
	}
	if options.entry != "" {
		resolved.entry = options.entry
	}
	if options.output != "" {
		resolved.output = options.output
	}
	if options.generate != nil {
		resolved.generate = *options.generate
	}
	if options.test != nil {
		resolved.test = *options.test
	}
	return resolved
}

func (c *CLI) runGoCommand(ctx context.Context, args []string) error {
	if err := printCommand(c.Stdout, "go", args); err != nil {
		return err
	}
	command := exec.CommandContext(ctx, "go", args...)
	command.Dir = c.WorkDir
	command.Stdout = c.Stdout
	command.Stderr = c.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("go %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

func printCommand(w io.Writer, name string, args []string) error {
	if w == nil {
		w = io.Discard
	}
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, name)
	for _, arg := range args {
		parts = append(parts, quoteCommandArg(arg))
	}
	_, err := fmt.Fprintln(w, strings.Join(parts, " "))
	return err
}

func quoteCommandArg(arg string) string {
	if arg == "" || strings.ContainsAny(arg, " \t\n\"'") {
		return strconv.Quote(arg)
	}
	return arg
}

func ensureOutputDir(workDir string, output string) error {
	dir := filepath.Dir(output)
	if dir == "." {
		return nil
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(workDir, dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create build output directory %q: %w", dir, err)
	}
	return nil
}
