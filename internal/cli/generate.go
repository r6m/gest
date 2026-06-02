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

	result, err := runGenerate(root, generateRunOptions{dryRun: options.dryRun, explain: options.explain})
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
	root    string
	dryRun  bool
	explain bool
}

func parseGenerateOptions(args []string) (generateOptions, error) {
	var options generateOptions
	flags := flag.NewFlagSet("generate", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&options.root, "root", "", "generation root")
	flags.BoolVar(&options.dryRun, "dry-run", false, "print changes without writing files")
	flags.BoolVar(&options.explain, "explain", false, "print parsed routes and skipped route-like methods")
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
	explain         bool
	diagnostics     []generator.Diagnostic
	explanation     generator.GenerationExplanation
}

type generateRunOptions struct {
	dryRun  bool
	explain bool
}

func runGenerate(root string, options generateRunOptions) (generateResult, error) {
	packages, err := generator.ScanPackages(root, generator.ScanOptions{})
	if err != nil {
		return generateResult{}, err
	}

	controllers, diagnostics, err := generator.ParseControllerRoutes(packages)
	if err != nil {
		return generateResult{}, err
	}
	listeners, listenerDiagnostics, err := generator.ParseEventListeners(packages)
	if err != nil {
		return generateResult{}, err
	}
	diagnostics = append(diagnostics, listenerDiagnostics...)

	files, err := generator.GenerateMetadataFiles(controllers)
	if err != nil {
		return generateResult{}, err
	}
	eventFiles, err := generator.GenerateEventMetadataFiles(listeners)
	if err != nil {
		return generateResult{}, err
	}
	files = append(files, eventFiles...)

	result := generateResult{
		scannedPackages: len(packages),
		dryRun:          options.dryRun,
		explain:         options.explain,
		diagnostics:     append([]generator.Diagnostic(nil), diagnostics...),
	}
	if options.explain {
		explanation, err := generator.ExplainGeneration(packages, controllers)
		if err != nil {
			return generateResult{}, err
		}
		result.explanation = explanation
	}
	if hasErrorDiagnostics(diagnostics) {
		return result, nil
	}

	if options.dryRun {
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
	if result.explain {
		if err := writeGenerateExplanation(w, result.explanation); err != nil {
			return err
		}
	}
	return nil
}

func writeGenerateExplanation(w io.Writer, explanation generator.GenerationExplanation) error {
	if _, err := fmt.Fprintln(w, "explain: parsed controllers/routes"); err != nil {
		return err
	}
	if len(explanation.Controllers) == 0 {
		if _, err := fmt.Fprintln(w, "  none"); err != nil {
			return err
		}
	}
	for _, controller := range explanation.Controllers {
		if _, err := fmt.Fprintf(w, "  controller %s %s\n", controller.TypeName, controller.BasePath); err != nil {
			return err
		}
		if len(controller.Routes) == 0 {
			if _, err := fmt.Fprintln(w, "    routes: none"); err != nil {
				return err
			}
		}
		for _, route := range controller.Routes {
			if _, err := fmt.Fprintf(w, "    %s %s -> %s\n", route.Method, route.Path, route.HandlerName); err != nil {
				return err
			}
		}
	}
	if _, err := fmt.Fprintln(w, "explain: rejected route-like methods"); err != nil {
		return err
	}
	if len(explanation.Rejected) == 0 {
		if _, err := fmt.Fprintln(w, "  none"); err != nil {
			return err
		}
	}
	for _, rejected := range explanation.Rejected {
		if _, err := fmt.Fprintf(w, "  %s.%s: %s\n", rejected.TypeName, rejected.HandlerName, rejected.Reason); err != nil {
			return err
		}
		if rejected.Hint != "" {
			if _, err := fmt.Fprintf(w, "    hint: %s\n", rejected.Hint); err != nil {
				return err
			}
		}
	}
	for _, diagnostic := range explanation.Hints {
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
