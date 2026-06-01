package generator

import (
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// ScanOptions configures package scanning.
type ScanOptions struct {
	IncludeTestdata bool
}

// Package describes a discovered Go package.
type Package struct {
	ImportPath string
	Name       string
	Dir        string
	Files      []string
}

// ScanPackages scans root for Go packages.
func ScanPackages(root string, options ScanOptions) ([]Package, error) {
	if root == "" {
		root = "."
	}

	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve scan root %q: %w", root, err)
	}
	info, err := os.Stat(absoluteRoot)
	if err != nil {
		return nil, fmt.Errorf("scan root %q: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("scan root %q is not a directory", root)
	}

	moduleRoot, modulePath := findModule(absoluteRoot)
	packagesByDir := make(map[string]*Package)

	err = filepath.WalkDir(absoluteRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if shouldIgnoreDir(entry.Name(), options) && path != absoluteRoot {
				return filepath.SkipDir
			}
			return nil
		}
		if shouldIgnoreFile(entry.Name()) {
			return nil
		}

		packageName, err := parsePackageName(path)
		if err != nil {
			return err
		}
		dir := filepath.Dir(path)
		pkg := packagesByDir[dir]
		if pkg == nil {
			pkg = &Package{
				ImportPath: importPath(moduleRoot, modulePath, dir),
				Name:       packageName,
				Dir:        dir,
			}
			packagesByDir[dir] = pkg
		}
		pkg.Files = append(pkg.Files, path)

		return nil
	})
	if err != nil {
		return nil, err
	}

	packages := make([]Package, 0, len(packagesByDir))
	for _, pkg := range packagesByDir {
		slices.Sort(pkg.Files)
		packages = append(packages, *pkg)
	}
	slices.SortFunc(packages, func(a Package, b Package) int {
		if a.Dir < b.Dir {
			return -1
		}
		if a.Dir > b.Dir {
			return 1
		}
		return 0
	})

	return packages, nil
}

func shouldIgnoreDir(name string, options ScanOptions) bool {
	switch name {
	case "vendor", ".git", ".gest", "node_modules":
		return true
	case "testdata":
		return !options.IncludeTestdata
	default:
		return false
	}
}

func shouldIgnoreFile(name string) bool {
	return !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_gest.gen.go")
}

func parsePackageName(path string) (string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.PackageClauseOnly)
	if err != nil {
		return "", fmt.Errorf("parse package clause %q: %w", path, err)
	}
	if file.Name == nil {
		return "", fmt.Errorf("parse package clause %q: missing package name", path)
	}
	return file.Name.Name, nil
}

func findModule(root string) (string, string) {
	for dir := root; ; dir = filepath.Dir(dir) {
		modulePath, err := readModulePath(filepath.Join(dir, "go.mod"))
		if err == nil {
			return dir, modulePath
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ""
		}
	}
}

func readModulePath(path string) (string, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(file), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	return "", nil
}

func importPath(moduleRoot string, modulePath string, dir string) string {
	if moduleRoot == "" || modulePath == "" {
		return ""
	}
	relative, err := filepath.Rel(moduleRoot, dir)
	if err != nil {
		return ""
	}
	if relative == "." {
		return modulePath
	}
	return modulePath + "/" + filepath.ToSlash(relative)
}
