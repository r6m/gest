package generator

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// Controller describes controller-level decorator metadata.
type Controller struct {
	Package  Package
	TypeName string
	BasePath string
	Tag      string
	File     string
	Line     int
	Column   int
}

// ParseControllers parses controller-level MVP decorators from scanned packages.
func ParseControllers(packages []Package) ([]Controller, []Diagnostic, error) {
	controllers := make([]Controller, 0)
	diagnostics := make([]Diagnostic, 0)

	for _, pkg := range packages {
		for _, file := range pkg.Files {
			fileControllers, fileDiagnostics, err := parseControllerFile(pkg, file)
			if err != nil {
				return nil, nil, err
			}
			controllers = append(controllers, fileControllers...)
			diagnostics = append(diagnostics, fileDiagnostics...)
		}
	}

	sortControllers(controllers)
	sortDiagnostics(diagnostics)
	return controllers, diagnostics, nil
}

func parseControllerFile(pkg Package, file string) ([]Controller, []Diagnostic, error) {
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, file, nil, parser.ParseComments)
	if err != nil {
		return nil, nil, fmt.Errorf("parse Go file %q: %w", file, err)
	}

	controllers := make([]Controller, 0)
	diagnostics := make([]Diagnostic, 0)
	for _, declaration := range parsed.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok {
			diagnostics = append(diagnostics, invalidTargetDiagnosticsForDeclaration(fileSet, declaration)...)
			continue
		}
		if general.Doc == nil {
			continue
		}

		decorators := decoratorsFromComments(fileSet, general.Doc)
		if len(decorators) == 0 {
			continue
		}

		typeName, isSingleType := singleTypeName(general)
		if !isSingleType {
			for _, decorator := range decorators {
				if isControllerDecorator(decorator.Name) {
					diagnostics = append(diagnostics, invalidTargetDiagnostic(decorator, typeName))
				}
			}
			continue
		}

		var controller *Controller
		var tag string
		for _, decorator := range decorators {
			switch decorator.Name {
			case "Controller":
				basePath, ok := parseSingleStringArgument(decorator.Raw)
				if !ok {
					diagnostics = append(diagnostics, invalidSyntaxDiagnostic(
						decorator,
						"@Controller requires a single string path argument",
						`use @Controller("/users")`,
					))
					continue
				}
				controller = &Controller{
					Package:  pkg,
					TypeName: typeName,
					BasePath: basePath,
					File:     file,
					Line:     decorator.Line,
					Column:   decorator.Column,
					Tag:      tag,
				}
			case "Tag":
				parsedTag, ok := parseSingleStringArgument(decorator.Raw)
				if !ok {
					diagnostics = append(diagnostics, invalidSyntaxDiagnostic(
						decorator,
						"@Tag requires a single string name argument",
						`use @Tag("Users")`,
					))
					continue
				}
				tag = parsedTag
				if controller == nil {
					controller = &Controller{
						Package:  pkg,
						TypeName: typeName,
						File:     file,
					}
				}
				controller.Tag = parsedTag
			default:
				diagnostics = append(diagnostics, unknownDecoratorDiagnostic(decorator))
			}
		}
		if controller != nil && controller.BasePath != "" {
			controllers = append(controllers, *controller)
		}
	}

	return controllers, diagnostics, nil
}

func invalidTargetDiagnosticsForDeclaration(fileSet *token.FileSet, declaration ast.Decl) []Diagnostic {
	var comments *ast.CommentGroup
	var target string
	switch typed := declaration.(type) {
	case *ast.FuncDecl:
		comments = typed.Doc
		if typed.Name != nil {
			target = typed.Name.Name
		}
	default:
		return nil
	}
	if comments == nil {
		return nil
	}

	diagnostics := make([]Diagnostic, 0)
	for _, decorator := range decoratorsFromComments(fileSet, comments) {
		if isControllerDecorator(decorator.Name) {
			diagnostics = append(diagnostics, invalidTargetDiagnostic(decorator, target))
		} else {
			diagnostics = append(diagnostics, unknownDecoratorDiagnostic(decorator))
		}
	}
	return diagnostics
}

type decorator struct {
	Name   string
	Raw    string
	File   string
	Line   int
	Column int
}

func decoratorsFromComments(fileSet *token.FileSet, comments *ast.CommentGroup) []decorator {
	decorators := make([]decorator, 0)
	for _, comment := range comments.List {
		text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
		if !strings.HasPrefix(text, "@") {
			continue
		}
		position := fileSet.Position(comment.Pos())
		name := decoratorName(text)
		decorators = append(decorators, decorator{
			Name:   name,
			Raw:    text,
			File:   position.Filename,
			Line:   position.Line,
			Column: position.Column,
		})
	}
	return decorators
}

func decoratorName(text string) string {
	text = strings.TrimPrefix(text, "@")
	end := strings.IndexFunc(text, func(r rune) bool {
		return r == '(' || r == ' ' || r == '\t'
	})
	if end == -1 {
		return text
	}
	return text[:end]
}

func singleTypeName(declaration *ast.GenDecl) (string, bool) {
	if declaration.Tok != token.TYPE || len(declaration.Specs) != 1 {
		return "", false
	}
	spec, ok := declaration.Specs[0].(*ast.TypeSpec)
	if !ok || spec.Name == nil {
		return "", false
	}
	return spec.Name.Name, true
}

func parseSingleStringArgument(raw string) (string, bool) {
	open := strings.Index(raw, "(")
	close := strings.LastIndex(raw, ")")
	if open == -1 || close != len(raw)-1 || close <= open {
		return "", false
	}
	argument := strings.TrimSpace(raw[open+1 : close])
	value, err := strconv.Unquote(argument)
	if err != nil {
		return "", false
	}
	return value, true
}

func isControllerDecorator(name string) bool {
	return name == "Controller" || name == "Tag"
}

func invalidSyntaxDiagnostic(decorator decorator, message string, hint string) Diagnostic {
	return Diagnostic{
		Severity: SeverityError,
		Code:     DiagnosticInvalidDecoratorSyntax,
		Message:  message,
		Hint:     hint,
		File:     decorator.File,
		Line:     decorator.Line,
		Column:   decorator.Column,
		Target:   decorator.Name,
	}
}

func invalidTargetDiagnostic(decorator decorator, target string) Diagnostic {
	if target == "" {
		target = "<declaration>"
	}
	return Diagnostic{
		Severity: SeverityError,
		Code:     DiagnosticInvalidTarget,
		Message:  "@" + decorator.Name + " can only apply to a single type declaration",
		Hint:     "move the decorator directly above one controller type",
		File:     decorator.File,
		Line:     decorator.Line,
		Column:   decorator.Column,
		Target:   target,
	}
}

func unknownDecoratorDiagnostic(decorator decorator) Diagnostic {
	return Diagnostic{
		Severity: SeverityError,
		Code:     DiagnosticUnknownDecorator,
		Message:  "unknown decorator @" + decorator.Name,
		Hint:     "supported controller decorators are @Controller and @Tag",
		File:     decorator.File,
		Line:     decorator.Line,
		Column:   decorator.Column,
		Target:   decorator.Name,
	}
}

func sortControllers(controllers []Controller) {
	slices.SortFunc(controllers, func(a Controller, b Controller) int {
		if controllerLess(a, b) {
			return -1
		}
		if controllerLess(b, a) {
			return 1
		}
		return 0
	})
}

func controllerLess(a Controller, b Controller) bool {
	if filepath.ToSlash(a.File) != filepath.ToSlash(b.File) {
		return filepath.ToSlash(a.File) < filepath.ToSlash(b.File)
	}
	if a.Line != b.Line {
		return a.Line < b.Line
	}
	return a.TypeName < b.TypeName
}
