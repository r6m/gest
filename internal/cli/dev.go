package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/r6m/gest/internal/config"
)

const devDebounce = 250 * time.Millisecond

func (c *CLI) runDev(ctx context.Context, args []string) error {
	options, err := parseDevOptions(args)
	if err != nil {
		return err
	}
	cfg, err := config.Load(c.WorkDir)
	if err != nil {
		return err
	}

	runner := &execDevRunner{
		workDir: c.WorkDir,
		stdout:  c.Stdout,
		stderr:  c.Stderr,
	}
	dev := newDevSession(c.WorkDir, cfg, options, runner, c.Stdout)
	if err := dev.rebuild(ctx); err != nil {
		return err
	}
	return dev.watch(ctx)
}

type devOptions struct {
	entry    string
	watch    []string
	ignore   []string
	test     *bool
	generate *bool
	race     bool
	tags     string
	env      string
}

func parseDevOptions(args []string) (devOptions, error) {
	var options devOptions
	var watch csvFlag
	var ignore csvFlag
	flags := flag.NewFlagSet("dev", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&options.entry, "entry", "", "entry package")
	flags.Var(&watch, "watch", "comma-separated watch paths")
	flags.Var(&ignore, "ignore", "comma-separated ignore paths or patterns")
	flags.BoolFunc("test", "run tests before building", func(valueText string) error {
		value := parseFlagBool(valueText)
		options.test = &value
		return nil
	})
	flags.BoolFunc("no-test", "skip tests before building", func(valueText string) error {
		value := !parseFlagBool(valueText)
		options.test = &value
		return nil
	})
	flags.BoolFunc("generate", "run generation before building", func(valueText string) error {
		value := parseFlagBool(valueText)
		options.generate = &value
		return nil
	})
	flags.BoolFunc("no-generate", "skip generation before building", func(valueText string) error {
		value := !parseFlagBool(valueText)
		options.generate = &value
		return nil
	})
	flags.BoolVar(&options.race, "race", false, "enable race detector")
	flags.StringVar(&options.tags, "tags", "", "go build tags")
	flags.StringVar(&options.env, "env", "", "env file path")
	if err := flags.Parse(args); err != nil {
		return devOptions{}, fmt.Errorf("parse dev flags: %w", err)
	}
	if flags.NArg() > 0 {
		return devOptions{}, fmt.Errorf("unexpected dev argument %q", flags.Arg(0))
	}
	options.watch = watch.values
	options.ignore = ignore.values
	return options, nil
}

type csvFlag struct {
	values []string
}

func (f *csvFlag) String() string {
	return strings.Join(f.values, ",")
}

func (f *csvFlag) Set(value string) error {
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			f.values = append(f.values, part)
		}
	}
	return nil
}

type devRunner interface {
	Generate(ctx context.Context, root string) error
	Test(ctx context.Context) error
	Build(ctx context.Context, args []string) error
	Start(ctx context.Context, binary string) (devProcess, error)
}

type devProcess interface {
	Stop(context.Context) error
}

type devSession struct {
	workDir string
	cfg     config.Config
	options devOptions
	runner  devRunner
	stdout  io.Writer
	current devProcess
	watcher *pollWatcher
}

func newDevSession(workDir string, cfg config.Config, options devOptions, runner devRunner, stdout io.Writer) *devSession {
	return &devSession{
		workDir: workDir,
		cfg:     cfg,
		options: options,
		runner:  runner,
		stdout:  stdout,
	}
}

func (s *devSession) rebuild(ctx context.Context) error {
	if err := s.prepare(ctx); err != nil {
		return err
	}

	binary := filepath.Join(s.workDir, ".gest", "tmp", "app")
	if err := ensureOutputDir(s.workDir, binary); err != nil {
		return err
	}
	buildArgs := s.buildArgs(binary)
	if err := s.runner.Build(ctx, buildArgs); err != nil {
		return fmt.Errorf("dev build failed: %w", err)
	}

	next, err := s.runner.Start(ctx, binary)
	if err != nil {
		return fmt.Errorf("dev start failed: %w", err)
	}
	if s.current != nil {
		if err := s.current.Stop(ctx); err != nil {
			return fmt.Errorf("dev stop previous process: %w", err)
		}
	}
	s.current = next
	_, _ = fmt.Fprintln(s.output(), "dev: app started")
	return nil
}

func (s *devSession) rebuildKeepingPrevious(ctx context.Context) {
	if err := s.rebuild(ctx); err != nil {
		_, _ = fmt.Fprintf(s.output(), "dev: %v; keeping previous process alive\n", err)
	}
}

func (s *devSession) prepare(ctx context.Context) error {
	if s.generateEnabled() {
		root := s.cfg.Generate.Root
		if !filepath.IsAbs(root) {
			root = filepath.Join(s.workDir, root)
		}
		if err := s.runner.Generate(ctx, filepath.Clean(root)); err != nil {
			return fmt.Errorf("dev generate failed: %w", err)
		}
	}
	if s.testEnabled() {
		if err := s.runner.Test(ctx); err != nil {
			return fmt.Errorf("dev test failed: %w", err)
		}
	}
	return nil
}

func (s *devSession) buildArgs(binary string) []string {
	args := []string{"build"}
	if s.cfg.Build.Trimpath {
		args = append(args, "-trimpath")
	}
	if s.options.race {
		args = append(args, "-race")
	}
	if s.options.tags != "" {
		args = append(args, "-tags", s.options.tags)
	}
	args = append(args, "-o", binary, s.entry())
	return args
}

func (s *devSession) entry() string {
	if s.options.entry != "" {
		return s.options.entry
	}
	return s.cfg.Build.Entry
}

func (s *devSession) generateEnabled() bool {
	if s.options.generate != nil {
		return *s.options.generate
	}
	return s.cfg.Build.Generate
}

func (s *devSession) testEnabled() bool {
	if s.options.test != nil {
		return *s.options.test
	}
	return s.cfg.Build.Test
}

func (s *devSession) output() io.Writer {
	if s.stdout == nil {
		return io.Discard
	}
	return s.stdout
}

func (s *devSession) watch(ctx context.Context) error {
	watcher := newPollWatcher(s.workDir, s.watchPaths(), s.ignorePatterns(), devDebounce)
	s.watcher = watcher
	return watcher.Run(ctx, func(paths []string) {
		_, _ = fmt.Fprintf(s.output(), "dev: change detected: %s\n", strings.Join(paths, ", "))
		s.rebuildKeepingPrevious(ctx)
	})
}

func (s *devSession) watchPaths() []string {
	if len(s.options.watch) > 0 {
		return append([]string(nil), s.options.watch...)
	}
	return []string{"cmd", "internal", "pkg", "config", ".env", "gest.yaml"}
}

func (s *devSession) ignorePatterns() []string {
	ignore := []string{"vendor", ".git", "tmp", ".gest", "*_gest.gen.go"}
	ignore = append(ignore, s.options.ignore...)
	return ignore
}

type execDevRunner struct {
	workDir string
	stdout  io.Writer
	stderr  io.Writer
}

func (r *execDevRunner) Generate(ctx context.Context, root string) error {
	result, err := runGenerate(root, false)
	if err != nil {
		return err
	}
	if err := writeGenerateOutput(r.stdout, result); err != nil {
		return err
	}
	if hasErrorDiagnostics(result.diagnostics) {
		return fmt.Errorf("generation failed")
	}
	return nil
}

func (r *execDevRunner) Test(ctx context.Context) error {
	return r.runGo(ctx, []string{"test", "./..."})
}

func (r *execDevRunner) Build(ctx context.Context, args []string) error {
	return r.runGo(ctx, args)
}

func (r *execDevRunner) Start(ctx context.Context, binary string) (devProcess, error) {
	_, _ = fmt.Fprintf(outputOrDiscard(r.stdout), "dev: start %s\n", binary)
	command := exec.CommandContext(ctx, binary)
	command.Dir = r.workDir
	command.Stdout = r.stdout
	command.Stderr = r.stderr
	if err := command.Start(); err != nil {
		return nil, err
	}
	return &execDevProcess{command: command}, nil
}

func (r *execDevRunner) runGo(ctx context.Context, args []string) error {
	if err := printCommand(r.stdout, "go", args); err != nil {
		return err
	}
	command := exec.CommandContext(ctx, "go", args...)
	command.Dir = r.workDir
	command.Stdout = r.stdout
	command.Stderr = r.stderr
	return command.Run()
}

type execDevProcess struct {
	command *exec.Cmd
}

func (p *execDevProcess) Stop(ctx context.Context) error {
	if p.command.Process == nil {
		return nil
	}
	if err := p.command.Process.Signal(os.Interrupt); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() {
		done <- p.command.Wait()
	}()
	select {
	case <-ctx.Done():
		_ = p.command.Process.Kill()
		return ctx.Err()
	case <-time.After(2 * time.Second):
		_ = p.command.Process.Kill()
		<-done
		return nil
	case <-done:
		return nil
	}
}

func outputOrDiscard(w io.Writer) io.Writer {
	if w == nil {
		return io.Discard
	}
	return w
}
