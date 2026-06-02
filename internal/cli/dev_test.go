package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/r6m/gest/internal/config"
)

func TestDevCommandRoutesToHandler(t *testing.T) {
	var called []string
	command := &CLI{
		Dev: recordHandler(&called, "dev"),
	}

	code := command.Run(context.Background(), []string{"dev"}, ioDiscard{}, ioDiscard{})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !reflect.DeepEqual(called, []string{"dev"}) {
		t.Fatalf("called = %#v, want dev", called)
	}
}

func TestDevInitialGenerateBuildStartSequence(t *testing.T) {
	runner := &fakeDevRunner{}
	session := newTestDevSession(t, runner, devOptions{})

	if err := session.rebuild(context.Background()); err != nil {
		t.Fatalf("rebuild returned error: %v", err)
	}

	want := []string{
		"generate:" + filepath.Join(session.workDir, "."),
		"test",
		"build:build -trimpath -o " + filepath.Join(session.workDir, ".gest", "tmp", "app") + " ./cmd/api",
		"start:" + filepath.Join(session.workDir, ".gest", "tmp", "app"),
	}
	if !reflect.DeepEqual(runner.events, want) {
		t.Fatalf("events = %#v, want %#v", runner.events, want)
	}
}

func TestDevFileChangeTriggersRebuild(t *testing.T) {
	runner := &fakeDevRunner{}
	session := newTestDevSession(t, runner, devOptions{})
	if err := session.rebuild(context.Background()); err != nil {
		t.Fatalf("initial rebuild returned error: %v", err)
	}

	session.rebuildKeepingPrevious(context.Background())

	if starts := countPrefix(runner.events, "start:"); starts != 2 {
		t.Fatalf("starts = %d, want 2; events %#v", starts, runner.events)
	}
	if stops := countPrefix(runner.events, "stop:"); stops != 1 {
		t.Fatalf("stops = %d, want 1; events %#v", stops, runner.events)
	}
}

func TestDevBuildFailureKeepsOldProcessAlive(t *testing.T) {
	var output bytes.Buffer
	runner := &fakeDevRunner{}
	session := newTestDevSession(t, runner, devOptions{})
	session.stdout = &output
	if err := session.rebuild(context.Background()); err != nil {
		t.Fatalf("initial rebuild returned error: %v", err)
	}
	runner.buildErr = errors.New("compile failed")

	session.rebuildKeepingPrevious(context.Background())

	if starts := countPrefix(runner.events, "start:"); starts != 1 {
		t.Fatalf("starts = %d, want 1; events %#v", starts, runner.events)
	}
	if stops := countPrefix(runner.events, "stop:"); stops != 0 {
		t.Fatalf("stops = %d, want 0; events %#v", stops, runner.events)
	}
	if !strings.Contains(output.String(), "compile failed") || !strings.Contains(output.String(), "keeping previous process alive") {
		t.Fatalf("output = %q, want failure diagnostic", output.String())
	}
}

func TestDevGeneratedFileChangeIsIgnored(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "internal/users/users.go", "package users\n")
	writeFile(t, root, "internal/users/users_gest.gen.go", "package users\n")

	watcher := newPollWatcher(root, []string{"internal"}, []string{"*_gest.gen.go"}, time.Millisecond)
	watcher.snapshot()
	writeFile(t, root, "internal/users/users_gest.gen.go", "package users\n// changed\n")

	if changed := watcher.Changed(); len(changed) != 0 {
		t.Fatalf("changed = %#v, want generated file ignored", changed)
	}
}

func TestDevDebounceSettingIsControlled(t *testing.T) {
	session := newTestDevSession(t, &fakeDevRunner{}, devOptions{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := session.watch(ctx); err == nil {
		t.Fatal("watch returned nil, want canceled context")
	}
	if session.watcher == nil {
		t.Fatal("watcher was not created")
	}
	if session.watcher.debounce != devDebounce {
		t.Fatalf("debounce = %s, want %s", session.watcher.debounce, devDebounce)
	}
}

func TestDevFlagsOverrideConfig(t *testing.T) {
	testFalse := false
	generateFalse := false
	options := devOptions{
		entry:    "./cmd/worker",
		test:     &testFalse,
		generate: &generateFalse,
		race:     true,
		tags:     "dev",
	}
	runner := &fakeDevRunner{}
	session := newTestDevSession(t, runner, options)

	if err := session.rebuild(context.Background()); err != nil {
		t.Fatalf("rebuild returned error: %v", err)
	}

	got := strings.Join(runner.events, "\n")
	if strings.Contains(got, "generate:") || strings.Contains(got, "test") {
		t.Fatalf("events = %#v, want generate and test skipped", runner.events)
	}
	wantBuild := "build:build -trimpath -race -tags dev -o " + filepath.Join(session.workDir, ".gest", "tmp", "app") + " ./cmd/worker"
	if !strings.Contains(got, wantBuild) {
		t.Fatalf("events = %#v, want %q", runner.events, wantBuild)
	}
}

func TestParseDevFlags(t *testing.T) {
	options, err := parseDevOptions([]string{
		"--entry", "./cmd/worker",
		"--watch", "cmd,internal",
		"--ignore", "dist,*.tmp",
		"--test",
		"--no-generate",
		"--race",
		"--tags", "sqlite",
		"--env", ".env.local",
	})
	if err != nil {
		t.Fatalf("parseDevOptions returned error: %v", err)
	}

	if options.entry != "./cmd/worker" || !options.race || options.tags != "sqlite" || options.env != ".env.local" {
		t.Fatalf("options = %#v, want flag values", options)
	}
	if options.test == nil || !*options.test {
		t.Fatalf("test option = %#v, want true", options.test)
	}
	if options.generate == nil || *options.generate {
		t.Fatalf("generate option = %#v, want false", options.generate)
	}
	if !reflect.DeepEqual(options.watch, []string{"cmd", "internal"}) {
		t.Fatalf("watch = %#v", options.watch)
	}
	if !reflect.DeepEqual(options.ignore, []string{"dist", "*.tmp"}) {
		t.Fatalf("ignore = %#v", options.ignore)
	}
}

func newTestDevSession(t *testing.T, runner *fakeDevRunner, options devOptions) *devSession {
	t.Helper()
	root := t.TempDir()
	cfg := config.Defaults()
	cfg.Build.Test = true
	session := newDevSession(root, cfg, options, runner, ioDiscard{})
	return session
}

type fakeDevRunner struct {
	events   []string
	buildErr error
}

func (r *fakeDevRunner) Generate(_ context.Context, root string) error {
	r.events = append(r.events, "generate:"+root)
	return nil
}

func (r *fakeDevRunner) Test(context.Context) error {
	r.events = append(r.events, "test")
	return nil
}

func (r *fakeDevRunner) Build(_ context.Context, args []string) error {
	r.events = append(r.events, "build:"+strings.Join(args, " "))
	return r.buildErr
}

func (r *fakeDevRunner) Start(_ context.Context, binary string) (devProcess, error) {
	r.events = append(r.events, "start:"+binary)
	return &fakeDevProcess{runner: r, binary: binary}, nil
}

type fakeDevProcess struct {
	runner *fakeDevRunner
	binary string
}

func (p *fakeDevProcess) Stop(context.Context) error {
	p.runner.events = append(p.runner.events, "stop:"+p.binary)
	return nil
}

func countPrefix(values []string, prefix string) int {
	count := 0
	for _, value := range values {
		if strings.HasPrefix(value, prefix) {
			count++
		}
	}
	return count
}

func writeFile(t *testing.T, root string, name string, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
}
