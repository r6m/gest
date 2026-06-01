package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

func (c *CLI) runGenerateModule(ctx context.Context, args []string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	options, err := parseModuleOptions(args)
	if err != nil {
		return err
	}
	modulePath, err := cleanModulePath(options.path)
	if err != nil {
		return err
	}

	result, err := c.generateModule(modulePath, options)
	if err != nil {
		return err
	}
	return writeModuleOutput(c.Stdout, result)
}

type moduleOptions struct {
	path         string
	dryRun       bool
	force        bool
	updateParent bool
}

func parseModuleOptions(args []string) (moduleOptions, error) {
	options := moduleOptions{updateParent: true}
	paths := make([]string, 0, 1)
	for _, arg := range args {
		switch arg {
		case "--dry-run":
			options.dryRun = true
		case "--force":
			options.force = true
		case "--no-update-parent":
			options.updateParent = false
		default:
			if strings.HasPrefix(arg, "-") {
				return moduleOptions{}, fmt.Errorf("unknown g module flag %q", arg)
			}
			paths = append(paths, arg)
		}
	}
	if len(paths) != 1 {
		return moduleOptions{}, errors.New("g module requires exactly one path")
	}
	options.path = paths[0]
	return options, nil
}

type moduleGenerateResult struct {
	created       []string
	updated       []string
	warnings      []string
	hints         []string
	dryRun        bool
	noParent      bool
	parentSkipped bool
}

func (c *CLI) generateModule(modulePath string, options moduleOptions) (moduleGenerateResult, error) {
	result := moduleGenerateResult{dryRun: options.dryRun, parentSkipped: !options.updateParent}
	target := moduleFilePath(c.WorkDir, modulePath)
	relativeTarget := slashRel(c.WorkDir, target)
	content, err := moduleFileContent(modulePath)
	if err != nil {
		return result, err
	}

	if _, err := os.Stat(target); err == nil && !options.force {
		return result, fmt.Errorf("%s already exists; use --force to overwrite", relativeTarget)
	} else if err != nil && !os.IsNotExist(err) {
		return result, err
	}

	result.created = append(result.created, relativeTarget)
	if !options.dryRun {
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return result, err
		}
		if err := os.WriteFile(target, content, 0o644); err != nil {
			return result, err
		}
	}

	if !options.updateParent {
		return result, nil
	}

	parent := findParentModule(c.WorkDir, modulePath)
	if parent == "" {
		result.noParent = true
		result.warnings = append(result.warnings, "parent module not found")
		result.hints = append(result.hints, "add "+moduleCall(modulePath)+" manually")
		return result, nil
	}

	relativeParent := slashRel(c.WorkDir, parent)
	if options.dryRun {
		result.updated = append(result.updated, relativeParent)
		return result, nil
	}
	updated, err := updateParentModule(parent, c.WorkDir, modulePath)
	if err != nil {
		return result, err
	}
	if updated {
		result.updated = append(result.updated, relativeParent)
	} else {
		result.warnings = append(result.warnings, "parent module already imports "+moduleCall(modulePath))
	}
	return result, nil
}

func cleanModulePath(raw string) (string, error) {
	raw = strings.Trim(raw, "/")
	if raw == "" || strings.Contains(raw, "..") {
		return "", fmt.Errorf("invalid module path %q", raw)
	}
	parts := strings.Split(raw, "/")
	for _, part := range parts {
		if !isIdentifier(part) {
			return "", fmt.Errorf("invalid module path segment %q", part)
		}
	}
	return strings.Join(parts, "/"), nil
}

func isIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for index, r := range value {
		if index == 0 && !unicode.IsLetter(r) && r != '_' {
			return false
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return true
}

func moduleFilePath(workDir string, modulePath string) string {
	parts := strings.Split(modulePath, "/")
	return filepath.Join(workDir, "internal", filepath.Join(parts...), parts[len(parts)-1]+".module.go")
}

func moduleFileContent(modulePath string) ([]byte, error) {
	parts := strings.Split(modulePath, "/")
	packageName := parts[len(parts)-1]
	moduleName := strings.Join(parts, ".")
	source := fmt.Sprintf(`package %s

import "github.com/r6m/gest"

type Options struct{}

func Module(options Options) gest.Module {
	return gest.NewModule(gest.ModuleConfig{
		Name: %q,
	})
}
`, packageName, moduleName)
	return format.Source([]byte(source))
}

func findParentModule(workDir string, modulePath string) string {
	parts := strings.Split(modulePath, "/")
	if len(parts) > 1 {
		parentParts := parts[:len(parts)-1]
		parent := filepath.Join(append([]string{workDir, "internal"}, parentParts...)...)
		path := filepath.Join(parent, parentParts[len(parentParts)-1]+".module.go")
		if fileExists(path) {
			return path
		}
	}
	rootApp := filepath.Join(workDir, "internal", "app", "app.module.go")
	if fileExists(rootApp) {
		return rootApp
	}
	return ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func updateParentModule(path string, workDir string, modulePath string) (bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	moduleImport, err := importPathFor(workDir, modulePath)
	if err != nil {
		return false, err
	}

	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, path, content, parser.ParseComments)
	if err != nil {
		return false, fmt.Errorf("parse parent module %s: %w", slashRel(workDir, path), err)
	}
	if parentHasModuleCall(parsed, modulePath) {
		return false, nil
	}

	edits := make([]textEdit, 0, 2)
	if !hasImport(parsed, moduleImport) {
		edit, err := importEdit(fileSet, parsed, moduleImport)
		if err != nil {
			return false, err
		}
		edits = append(edits, edit)
	}
	edit, err := importsEdit(fileSet, parsed, modulePath)
	if err != nil {
		return false, err
	}
	edits = append(edits, edit)

	updated := applyEdits(content, edits)
	formatted, err := format.Source(updated)
	if err != nil {
		return false, fmt.Errorf("format parent module %s: %w", slashRel(workDir, path), err)
	}
	if bytes.Equal(content, formatted) {
		return false, nil
	}
	if err := os.WriteFile(path, formatted, 0o644); err != nil {
		return false, err
	}
	return true, nil
}

type textEdit struct {
	offset int
	text   string
}

func applyEdits(content []byte, edits []textEdit) []byte {
	slices.SortFunc(edits, func(a textEdit, b textEdit) int {
		return b.offset - a.offset
	})
	updated := append([]byte(nil), content...)
	for _, edit := range edits {
		updated = append(updated[:edit.offset], append([]byte(edit.text), updated[edit.offset:]...)...)
	}
	return updated
}

func hasImport(file *ast.File, importPath string) bool {
	quoted := strconvQuote(importPath)
	for _, spec := range file.Imports {
		if spec.Path != nil && spec.Path.Value == quoted {
			return true
		}
	}
	return false
}

func importEdit(fileSet *token.FileSet, file *ast.File, importPath string) (textEdit, error) {
	quoted := strconvQuote(importPath)
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.IMPORT {
			continue
		}
		if general.Lparen.IsValid() {
			return textEdit{
				offset: fileSet.Position(general.Rparen).Offset,
				text:   "\t" + quoted + "\n",
			}, nil
		}
		return textEdit{
			offset: fileSet.Position(general.End()).Offset,
			text:   "\n\nimport " + quoted,
		}, nil
	}
	return textEdit{
		offset: fileSet.Position(file.Name.End()).Offset,
		text:   "\n\nimport " + quoted,
	}, nil
}

func importsEdit(fileSet *token.FileSet, file *ast.File, modulePath string) (textEdit, error) {
	call := moduleCall(modulePath)
	var moduleConfig *ast.CompositeLit
	ast.Inspect(file, func(node ast.Node) bool {
		if moduleConfig != nil {
			return false
		}
		lit, ok := node.(*ast.CompositeLit)
		if !ok || !isGestModuleConfig(lit.Type) {
			return true
		}
		moduleConfig = lit
		return false
	})
	if moduleConfig == nil {
		return textEdit{}, errors.New("parent module does not contain gest.ModuleConfig")
	}

	for _, element := range moduleConfig.Elts {
		kv, ok := element.(*ast.KeyValueExpr)
		if !ok || identName(kv.Key) != "Imports" {
			continue
		}
		callExpr, ok := kv.Value.(*ast.CallExpr)
		if !ok || !isGestImports(callExpr.Fun) {
			return textEdit{}, errors.New("parent module Imports field is not gest.Imports(...)")
		}
		offset := fileSet.Position(callExpr.Rparen).Offset
		prefix := ""
		if len(callExpr.Args) > 0 {
			prefix = "\n"
		}
		return textEdit{offset: offset, text: prefix + "\t\t\t" + call + ","}, nil
	}

	offset := fileSet.Position(moduleConfig.Rbrace).Offset
	return textEdit{
		offset: offset,
		text:   "\t\tImports: gest.Imports(\n\t\t\t" + call + ",\n\t\t),\n",
	}, nil
}

func isGestModuleConfig(expr ast.Expr) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	return ok && selector.Sel != nil && selector.Sel.Name == "ModuleConfig" && identName(selector.X) == "gest"
}

func isGestImports(expr ast.Expr) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	return ok && selector.Sel != nil && selector.Sel.Name == "Imports" && identName(selector.X) == "gest"
}

func identName(expr ast.Expr) string {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return ""
	}
	return ident.Name
}

func parentHasModuleCall(file *ast.File, modulePath string) bool {
	call := moduleCall(modulePath)
	found := false
	ast.Inspect(file, func(node ast.Node) bool {
		if found {
			return false
		}
		expr, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		found = callExprString(expr) == call
		return !found
	})
	return found
}

func callExprString(expr ast.Expr) string {
	var buffer bytes.Buffer
	if err := format.Node(&buffer, token.NewFileSet(), expr); err != nil {
		return ""
	}
	return buffer.String()
}

func moduleCall(modulePath string) string {
	parts := strings.Split(modulePath, "/")
	packageName := parts[len(parts)-1]
	return packageName + ".Module(" + packageName + ".Options{})"
}

func importPathFor(workDir string, modulePath string) (string, error) {
	moduleName, err := readGoModulePath(filepath.Join(workDir, "go.mod"))
	if err != nil {
		return "", err
	}
	return moduleName + "/internal/" + modulePath, nil
}

func readGoModulePath(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	return "", fmt.Errorf("%s does not declare a module path", path)
}

func slashRel(root string, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}

func strconvQuote(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

func writeModuleOutput(w io.Writer, result moduleGenerateResult) error {
	if w == nil {
		w = io.Discard
	}
	prefix := ""
	if result.dryRun {
		prefix = "DRY-RUN "
	}
	for _, path := range result.created {
		if _, err := fmt.Fprintf(w, "%sCREATE %s\n", prefix, path); err != nil {
			return err
		}
	}
	for _, path := range result.updated {
		if _, err := fmt.Fprintf(w, "%sUPDATE %s\n", prefix, path); err != nil {
			return err
		}
	}
	if result.parentSkipped {
		if _, err := fmt.Fprintln(w, "SKIP parent module update"); err != nil {
			return err
		}
	}
	for _, warning := range result.warnings {
		if _, err := fmt.Fprintf(w, "WARN %s\n", warning); err != nil {
			return err
		}
	}
	for _, hint := range result.hints {
		if _, err := fmt.Fprintf(w, "HINT %s\n", hint); err != nil {
			return err
		}
	}
	return nil
}
