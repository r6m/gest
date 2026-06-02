package generator

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// GenerationExplanation is a concise, deterministic generation analysis.
type GenerationExplanation struct {
	Controllers []ControllerExplanation
	Rejected    []RejectedRouteExplanation
	Hints       []Diagnostic
}

// ControllerExplanation describes parsed generated-controller metadata.
type ControllerExplanation struct {
	TypeName string
	BasePath string
	File     string
	Line     int
	Routes   []RouteExplanation
}

// RouteExplanation describes a parsed generated route.
type RouteExplanation struct {
	HandlerName string
	Method      string
	Path        string
	File        string
	Line        int
}

// RejectedRouteExplanation describes a route-like method skipped by generation.
type RejectedRouteExplanation struct {
	TypeName    string
	HandlerName string
	File        string
	Line        int
	Reason      string
	Hint        string
}

// ExplainGeneration returns parsed routes and framework-aware skipped-route hints.
func ExplainGeneration(packages []Package, controllers []Controller) (GenerationExplanation, error) {
	explanation := GenerationExplanation{
		Controllers: explainControllers(controllers),
	}
	controllerTypes := make(map[string]Controller)
	for _, controller := range controllers {
		controllerTypes[controller.TypeName] = controller
	}
	providedConstructors, err := providedControllerConstructors(packages)
	if err != nil {
		return GenerationExplanation{}, err
	}
	for _, controller := range controllers {
		constructor := "New" + controller.TypeName
		if !providedConstructors[constructor] {
			explanation.Hints = append(explanation.Hints, Diagnostic{
				Severity: SeverityInfo,
				Code:     DiagnosticControllerNotProvided,
				Message:  "generated controller " + controller.TypeName + " is not provided in a module",
				Hint:     "add gest.Controller(" + constructor + ") to this package module, or import a module that provides " + controller.TypeName,
				File:     controller.File,
				Line:     controller.Line,
				Target:   controller.TypeName,
			})
		}
	}

	for _, pkg := range packages {
		for _, file := range pkg.Files {
			rejected, diagnostics, err := explainRejectedRoutes(pkg, file, controllerTypes)
			if err != nil {
				return GenerationExplanation{}, err
			}
			explanation.Rejected = append(explanation.Rejected, rejected...)
			explanation.Hints = append(explanation.Hints, diagnostics...)
		}
	}

	sortControllerExplanations(explanation.Controllers)
	sortRejectedRoutes(explanation.Rejected)
	sortDiagnostics(explanation.Hints)
	return explanation, nil
}

func explainControllers(controllers []Controller) []ControllerExplanation {
	explanations := make([]ControllerExplanation, 0, len(controllers))
	for _, controller := range controllers {
		item := ControllerExplanation{
			TypeName: controller.TypeName,
			BasePath: controller.BasePath,
			File:     controller.File,
			Line:     controller.Line,
			Routes:   make([]RouteExplanation, 0, len(controller.Routes)),
		}
		for _, route := range controller.Routes {
			item.Routes = append(item.Routes, RouteExplanation{
				HandlerName: route.HandlerName,
				Method:      route.Method,
				Path:        route.Path,
				File:        route.File,
				Line:        route.Line,
			})
		}
		explanations = append(explanations, item)
	}
	return explanations
}

func providedControllerConstructors(packages []Package) (map[string]bool, error) {
	provided := make(map[string]bool)
	for _, pkg := range packages {
		for _, file := range pkg.Files {
			content, err := os.ReadFile(file)
			if err != nil {
				return nil, fmt.Errorf("read %q: %w", file, err)
			}
			text := string(content)
			search := "gest.Controller("
			for {
				index := strings.Index(text, search)
				if index == -1 {
					break
				}
				after := text[index+len(search):]
				name := leadingIdentifier(strings.TrimLeft(after, " \t\r\n"))
				if name != "" {
					provided[name] = true
				}
				text = after
			}
		}
	}
	return provided, nil
}

func explainRejectedRoutes(pkg Package, file string, controllerTypes map[string]Controller) ([]RejectedRouteExplanation, []Diagnostic, error) {
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, file, nil, parser.ParseComments)
	if err != nil {
		return nil, nil, fmt.Errorf("parse Go file %q: %w", file, err)
	}
	rejected := []RejectedRouteExplanation{}
	diagnostics := []Diagnostic{}
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Recv == nil || function.Name == nil {
			continue
		}
		receiver := receiverTypeName(function)
		if receiver == "" {
			continue
		}
		position := fileSet.Position(function.Pos())
		decorators := []decorator{}
		if function.Doc != nil {
			decorators = decoratorsFromComments(fileSet, function.Doc)
		}
		hasRouteDecorator := hasHTTPMethodDecorator(decorators)
		_, isController := controllerTypes[receiver]
		if hasRouteDecorator && !isController {
			rejected = append(rejected, RejectedRouteExplanation{
				TypeName:    receiver,
				HandlerName: function.Name.Name,
				File:        position.Filename,
				Line:        position.Line,
				Reason:      "method has a route decorator but receiver type " + receiver + " is not a parsed controller",
				Hint:        "add @Controller(...) to " + receiver + " or move the route method onto a controller type",
			})
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityInfo,
				Code:     DiagnosticDetachedRoute,
				Message:  function.Name.Name + " has a route decorator but " + receiver + " is not a parsed controller",
				Hint:     "add @Controller(...) to " + receiver + " or move the route method onto a controller type",
				File:     position.Filename,
				Line:     position.Line,
				Target:   function.Name.Name,
			})
			continue
		}
		if hasRouteDecorator || !isRouteLikeMethod(function.Name.Name) {
			continue
		}
		if _, ok := validateHandlerSignature(function); !ok {
			continue
		}
		rejected = append(rejected, RejectedRouteExplanation{
			TypeName:    receiver,
			HandlerName: function.Name.Name,
			File:        position.Filename,
			Line:        position.Line,
			Reason:      "method has a valid Gest handler signature but no route decorator",
			Hint:        "add @Get, @Post, @Put, @Patch, or @Delete above " + function.Name.Name + " if it should be generated as a route",
		})
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityInfo,
			Code:     DiagnosticSkippedRoute,
			Message:  function.Name.Name + " has a valid Gest handler signature but no route decorator",
			Hint:     "add @Get, @Post, @Put, @Patch, or @Delete above " + function.Name.Name + " if it should be generated as a route",
			File:     position.Filename,
			Line:     position.Line,
			Target:   function.Name.Name,
		})
		if strings.HasSuffix(receiver, "Controller") {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityInfo,
				Code:     DiagnosticLikelyMissingModuleImport,
				Message:  receiver + " looks like a controller type but has no @Controller decorator",
				Hint:     "add @Controller(...) above " + receiver + "; if it already has a module, import that module from the app module",
				File:     position.Filename,
				Line:     position.Line,
				Target:   receiver,
			})
		}
	}
	return rejected, diagnostics, nil
}

func hasHTTPMethodDecorator(decorators []decorator) bool {
	for _, decorator := range decorators {
		if isHTTPMethodDecorator(decorator.Name) {
			return true
		}
	}
	return false
}

func isRouteLikeMethod(name string) bool {
	prefixes := []string{"Get", "List", "Find", "Create", "Post", "Put", "Patch", "Update", "Delete"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func leadingIdentifier(value string) string {
	for index, r := range value {
		valid := r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (index > 0 && r >= '0' && r <= '9')
		if !valid {
			return value[:index]
		}
	}
	return value
}

func sortControllerExplanations(controllers []ControllerExplanation) {
	slices.SortFunc(controllers, func(a ControllerExplanation, b ControllerExplanation) int {
		if filepath.ToSlash(a.File) != filepath.ToSlash(b.File) {
			return strings.Compare(filepath.ToSlash(a.File), filepath.ToSlash(b.File))
		}
		if a.Line != b.Line {
			return a.Line - b.Line
		}
		return strings.Compare(a.TypeName, b.TypeName)
	})
	for i := range controllers {
		slices.SortFunc(controllers[i].Routes, func(a RouteExplanation, b RouteExplanation) int {
			if filepath.ToSlash(a.File) != filepath.ToSlash(b.File) {
				return strings.Compare(filepath.ToSlash(a.File), filepath.ToSlash(b.File))
			}
			if a.Line != b.Line {
				return a.Line - b.Line
			}
			return strings.Compare(a.HandlerName, b.HandlerName)
		})
	}
}

func sortRejectedRoutes(routes []RejectedRouteExplanation) {
	slices.SortFunc(routes, func(a RejectedRouteExplanation, b RejectedRouteExplanation) int {
		if filepath.ToSlash(a.File) != filepath.ToSlash(b.File) {
			return strings.Compare(filepath.ToSlash(a.File), filepath.ToSlash(b.File))
		}
		if a.Line != b.Line {
			return a.Line - b.Line
		}
		return strings.Compare(a.HandlerName, b.HandlerName)
	})
}
