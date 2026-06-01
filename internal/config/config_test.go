package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMissingConfigUsesDefaults(t *testing.T) {
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("load missing config: %v", err)
	}

	want := Config{
		Entry: "./cmd/api",
		Router: RouterConfig{
			Adapter: "chi",
		},
		Generate: GenerateConfig{
			Root:    ".",
			OpenAPI: false,
		},
		Build: BuildConfig{
			Output:   "bin/app",
			Entry:    "./cmd/api",
			Generate: true,
			Test:     false,
			Trimpath: true,
		},
	}
	if cfg != want {
		t.Fatalf("unexpected defaults:\nwant: %#v\n got: %#v", want, cfg)
	}
}

func TestLoadPartialConfigMergesDefaults(t *testing.T) {
	dir := writeConfig(t, `
project:
  name: notes
generate:
  root: ./internal
`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Project.Name != "notes" {
		t.Fatalf("expected project name notes, got %q", cfg.Project.Name)
	}
	if cfg.Entry != "./cmd/api" {
		t.Fatalf("expected default entry, got %q", cfg.Entry)
	}
	if cfg.Generate.Root != "./internal" {
		t.Fatalf("expected generate root override, got %q", cfg.Generate.Root)
	}
	if cfg.Generate.OpenAPI {
		t.Fatal("expected generate.openapi default false")
	}
	if cfg.Build.Output != "bin/notes" {
		t.Fatalf("expected project-derived build output, got %q", cfg.Build.Output)
	}
	if cfg.Build.Entry != "./cmd/api" {
		t.Fatalf("expected build entry default, got %q", cfg.Build.Entry)
	}
	if !cfg.Build.Generate {
		t.Fatal("expected build.generate default true")
	}
	if cfg.Build.Test {
		t.Fatal("expected build.test default false")
	}
	if !cfg.Build.Trimpath {
		t.Fatal("expected build.trimpath default true")
	}
}

func TestLoadInvalidYAMLReturnsUsefulError(t *testing.T) {
	dir := writeConfig(t, "project:\n  name: [")

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); !strings.Contains(got, "parse") || !strings.Contains(got, "gest.yaml") {
		t.Fatalf("expected parse error with file name, got %q", got)
	}
}

func TestLoadInvalidFieldTypeReturnsUsefulError(t *testing.T) {
	dir := writeConfig(t, `
build:
  trimpath: 123
`)

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); !strings.Contains(got, "parse") || !strings.Contains(got, "gest.yaml") || !strings.Contains(got, "cannot unmarshal") {
		t.Fatalf("expected useful parse error, got %q", got)
	}
}

func TestBuildEntryInheritsTopLevelEntry(t *testing.T) {
	dir := writeConfig(t, `
entry: ./cmd/server
`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Build.Entry != "./cmd/server" {
		t.Fatalf("expected build entry to inherit top-level entry, got %q", cfg.Build.Entry)
	}
}

func TestExplicitFieldsOverrideDefaults(t *testing.T) {
	dir := writeConfig(t, `
project:
  name: api
entry: ./cmd/http
router:
  adapter: custom
generate:
  root: ./app
  openapi: true
build:
  output: dist/server
  entry: ./cmd/worker
  generate: false
  test: true
  trimpath: false
`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	want := Config{
		Project: ProjectConfig{
			Name: "api",
		},
		Entry: "./cmd/http",
		Router: RouterConfig{
			Adapter: "custom",
		},
		Generate: GenerateConfig{
			Root:    "./app",
			OpenAPI: true,
		},
		Build: BuildConfig{
			Output:   "dist/server",
			Entry:    "./cmd/worker",
			Generate: false,
			Test:     true,
			Trimpath: false,
		},
	}
	if cfg != want {
		t.Fatalf("unexpected config:\nwant: %#v\n got: %#v", want, cfg)
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, fileName)
	if err := os.WriteFile(path, []byte(strings.TrimLeft(content, "\n")), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	return dir
}
