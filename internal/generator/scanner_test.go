package generator

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestScanPackagesFindsMultiplePackages(t *testing.T) {
	root := newFixture(t, map[string]string{
		"go.mod":              "module example.test/app\n\ngo 1.26.2\n",
		"cmd/api/main.go":     "package main\n",
		"internal/users/a.go": "package users\n",
		"internal/users/b.go": "package users\n",
	})

	packages, err := ScanPackages(root, ScanOptions{})
	if err != nil {
		t.Fatalf("ScanPackages returned error: %v", err)
	}

	got := packageSummaries(root, packages)
	want := []packageSummary{
		{
			ImportPath: "example.test/app/cmd/api",
			Name:       "main",
			Dir:        "cmd/api",
			Files:      []string{"main.go"},
		},
		{
			ImportPath: "example.test/app/internal/users",
			Name:       "users",
			Dir:        "internal/users",
			Files:      []string{"a.go", "b.go"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("packages = %#v, want %#v", got, want)
	}
}

func TestScanPackagesIgnoresGeneratedFilesAndIrrelevantDirectories(t *testing.T) {
	root := newFixture(t, map[string]string{
		"go.mod":                      "module example.test/app\n\ngo 1.26.2\n",
		"app/app.go":                  "package app\n",
		"app/app_gest.gen.go":         "package app\n",
		"app/readme.md":               "# ignored\n",
		"vendor/pkg/vendor.go":        "package vendor\n",
		".git/hooks/hook.go":          "package hooks\n",
		".gest/cache/cache.go":        "package cache\n",
		"node_modules/pkg/module.go":  "package module\n",
		"testdata/fixture/fixture.go": "package fixture\n",
	})

	packages, err := ScanPackages(root, ScanOptions{})
	if err != nil {
		t.Fatalf("ScanPackages returned error: %v", err)
	}

	got := packageSummaries(root, packages)
	want := []packageSummary{
		{
			ImportPath: "example.test/app/app",
			Name:       "app",
			Dir:        "app",
			Files:      []string{"app.go"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("packages = %#v, want %#v", got, want)
	}
}

func TestScanPackagesCanIncludeTestdata(t *testing.T) {
	root := newFixture(t, map[string]string{
		"go.mod":                      "module example.test/app\n\ngo 1.26.2\n",
		"app/app.go":                  "package app\n",
		"testdata/fixture/fixture.go": "package fixture\n",
	})

	packages, err := ScanPackages(root, ScanOptions{IncludeTestdata: true})
	if err != nil {
		t.Fatalf("ScanPackages returned error: %v", err)
	}

	got := packageSummaries(root, packages)
	want := []packageSummary{
		{
			ImportPath: "example.test/app/app",
			Name:       "app",
			Dir:        "app",
			Files:      []string{"app.go"},
		},
		{
			ImportPath: "example.test/app/testdata/fixture",
			Name:       "fixture",
			Dir:        "testdata/fixture",
			Files:      []string{"fixture.go"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("packages = %#v, want %#v", got, want)
	}
}

func TestScanPackagesOrderingIsDeterministic(t *testing.T) {
	root := newFixture(t, map[string]string{
		"go.mod":       "module example.test/app\n\ngo 1.26.2\n",
		"z/z.go":       "package z\n",
		"a/c.go":       "package a\n",
		"a/a.go":       "package a\n",
		"m/m.go":       "package m\n",
		"m/ignore.txt": "ignored\n",
	})

	first, err := ScanPackages(root, ScanOptions{})
	if err != nil {
		t.Fatalf("first ScanPackages returned error: %v", err)
	}
	second, err := ScanPackages(root, ScanOptions{})
	if err != nil {
		t.Fatalf("second ScanPackages returned error: %v", err)
	}

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("scan output changed between runs: first %#v, second %#v", first, second)
	}
	got := packageSummaries(root, first)
	wantDirs := []string{"a", "m", "z"}
	for i, want := range wantDirs {
		if got[i].Dir != want {
			t.Fatalf("package %d dir = %q, want %q", i, got[i].Dir, want)
		}
	}
	if !reflect.DeepEqual(got[0].Files, []string{"a.go", "c.go"}) {
		t.Fatalf("files = %#v, want sorted files", got[0].Files)
	}
}

func TestScanPackagesReturnsUsefulErrorForMissingRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing")

	_, err := ScanPackages(root, ScanOptions{})
	if err == nil {
		t.Fatal("ScanPackages returned nil error, want missing root error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("error = %v, want wrapped os.ErrNotExist", err)
	}
}

type packageSummary struct {
	ImportPath string
	Name       string
	Dir        string
	Files      []string
}

func packageSummaries(root string, packages []Package) []packageSummary {
	summaries := make([]packageSummary, 0, len(packages))
	for _, pkg := range packages {
		relativeDir, err := filepath.Rel(root, pkg.Dir)
		if err != nil {
			relativeDir = pkg.Dir
		}
		files := make([]string, 0, len(pkg.Files))
		for _, file := range pkg.Files {
			files = append(files, filepath.Base(file))
		}
		summaries = append(summaries, packageSummary{
			ImportPath: pkg.ImportPath,
			Name:       pkg.Name,
			Dir:        filepath.ToSlash(relativeDir),
			Files:      files,
		})
	}
	return summaries
}

func newFixture(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) returned error: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile(%q) returned error: %v", path, err)
		}
	}
	return root
}
