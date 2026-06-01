package cli

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/r6m/gest/internal/config"
	"github.com/r6m/gest/internal/generator"
)

func (c *CLI) runGenerate(ctx context.Context, args []string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	options, err := parseGenerateOptions(args)
	if err != nil {
		return err
	}

	cfg, err := config.Load(c.WorkDir)
	if err != nil {
		return err
	}

	root := cfg.Generate.Root
	if options.root != "" {
		root = options.root
	}
	if !filepath.IsAbs(root) {
		root = filepath.Join(c.WorkDir, root)
	}
	root = filepath.Clean(root)

	result, err := runGenerate(root, options.dryRun)
	if err != nil {
		return err
	}
	if err := writeGenerateOutput(c.Stdout, result); err != nil {
		return err
	}
	if hasErrorDiagnostics(result.diagnostics) {
		return errors.New("generation failed")
	}

	return nil
}

type generateOptions struct {
	root   string
	dryRun bool
}

func parseGenerateOptions(args []string) (generateOptions, error) {
	var options generateOptions
	flags := flag.NewFlagSet("generate", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&options.root, "root", "", "generation root")
	flags.BoolVar(&options.dryRun, "dry-run", false, "print changes without writing files")
	if err := flags.Parse(args); err != nil {
		return generateOptions{}, fmt.Errorf("parse generate flags: %w", err)
	}
	if flags.NArg() > 0 {
		return generateOptions{}, fmt.Errorf("unexpected generate argument %q", flags.Arg(0))
	}
	return options, nil
}

type generateResult struct {
	scannedPackages int
	generated       int
	updated         int
	skipped         int
	dryRun          bool
	diagnostics     []generator.Diagnostic
}

func runGenerate(root string, dryRun bool) (generateResult, error) {
	packages, err := generator.ScanPackages(root, generator.ScanOptions{})
	if err != nil {
		return generateResult{}, err
	}

	controllers, diagnostics, err := generator.ParseControllerRoutes(packages)
	if err != nil {
		return generateResult{}, err
	}

	files, err := generator.GenerateMetadataFiles(controllers)
	if err != nil {
		return generateResult{}, err
	}

	result := generateResult{
		scannedPackages: len(packages),
		dryRun:          dryRun,
		diagnostics:     append([]generator.Diagnostic(nil), diagnostics...),
	}
	if hasErrorDiagnostics(diagnostics) {
		return result, nil
	}

	if dryRun {
		countDryRunChanges(&result, files)
		return result, nil
	}

	existed := existingGeneratedFiles(files)
	writeResults, writeDiagnostics := generator.WriteGeneratedFiles(files)
	result.diagnostics = append(result.diagnostics, writeDiagnostics...)
	for _, writeResult := range writeResults {
		if !writeResult.Written {
			result.skipped++
			continue
		}
		if existed[writeResult.Path] {
			result.updated++
		} else {
			result.generated++
		}
	}

	return result, nil
}

func existingGeneratedFiles(files []generator.GeneratedFile) map[string]bool {
	existed := make(map[string]bool, len(files))
	for _, file := range files {
		if _, err := os.Stat(file.Path); err == nil {
			existed[file.Path] = true
		}
	}
	return existed
}

func countDryRunChanges(result *generateResult, files []generator.GeneratedFile) {
	for _, file := range files {
		existing, err := os.ReadFile(file.Path)
		if err == nil && bytes.Equal(existing, file.Content) {
			result.skipped++
			continue
		}
		if err == nil {
			result.updated++
			continue
		}
		result.generated++
	}
}

func writeGenerateOutput(w io.Writer, result generateResult) error {
	if w == nil {
		w = io.Discard
	}
	prefix := ""
	if result.dryRun {
		prefix = "dry-run "
	}
	if _, err := fmt.Fprintf(w, "scanned packages: %d\n", result.scannedPackages); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		w,
		"%sgenerated: %d updated: %d skipped: %d\n",
		prefix,
		result.generated,
		result.updated,
		result.skipped,
	); err != nil {
		return err
	}
	for _, diagnostic := range result.diagnostics {
		if _, err := fmt.Fprintf(w, "%s\n", diagnostic.Error()); err != nil {
			return err
		}
		if diagnostic.Hint != "" {
			if _, err := fmt.Fprintf(w, "hint: %s\n", diagnostic.Hint); err != nil {
				return err
			}
		}
	}
	return nil
}

func hasErrorDiagnostics(diagnostics []generator.Diagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == generator.SeverityError {
			return true
		}
	}
	return false
}
