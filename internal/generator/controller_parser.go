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
	Hidden   bool
	File     string
	Line     int
	Column   int
	Guards   []GuardReference
	Routes   []Route
}

// Route describes method-level route decorator metadata.
type Route struct {
	Method       string
	Path         string
	HandlerName  string
	RequestType  string
	ResponseType string
	Statuses     []int
	Summary      string
	Description  string
	Hidden       bool
	Guards       []GuardReference
	File         string
	Line         int
	Column       int
}

// GuardReference describes a parsed @Use(pkg.Symbol) route component reference.
type GuardReference struct {
	Alias      string
	Symbol     string
	ImportPath string
	File       string
	Line       int
	Column     int
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

	controllers, diagnostics := parseControllersFromAST(pkg, file, fileSet, parsed, true)
	return controllers, diagnostics, nil
}

// ParseControllerRoutes parses controller and route decorators from scanned packages.
func ParseControllerRoutes(packages []Package) ([]Controller, []Diagnostic, error) {
	controllers := make([]Controller, 0)
	diagnostics := make([]Diagnostic, 0)

	for _, pkg := range packages {
		for _, file := range pkg.Files {
			fileControllers, fileDiagnostics, err := parseControllerRoutesFile(pkg, file)
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

func parseControllerRoutesFile(pkg Package, file string) ([]Controller, []Diagnostic, error) {
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, file, nil, parser.ParseComments)
	if err != nil {
		return nil, nil, fmt.Errorf("parse Go file %q: %w", file, err)
	}

	controllers, diagnostics := parseControllersFromAST(pkg, file, fileSet, parsed, false)
	imports := importAliases(parsed)
	controllersByType := make(map[string]*Controller)
	for i := range controllers {
		controllersByType[controllers[i].TypeName] = &controllers[i]
	}

	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Doc == nil {
			continue
		}
		decorators := decoratorsFromComments(fileSet, function.Doc)
		if len(decorators) == 0 || !hasRouteRelevantDecorator(decorators) {
			continue
		}

		receiver := receiverTypeName(function)
		controller := controllersByType[receiver]
		if controller == nil {
			for _, decorator := range decorators {
				if isRouteRelevantDecorator(decorator.Name) {
					diagnostics = append(diagnostics, invalidRouteTargetDiagnostic(decorator, function.Name.Name))
				}
			}
			continue
		}

		route, routeDiagnostics := parseRouteDecorators(function.Name.Name, decorators, imports)
		diagnostics = append(diagnostics, routeDiagnostics...)
		if route != nil {
			signature, ok := validateHandlerSignature(function)
			if !ok {
				diagnostics = append(diagnostics, invalidHandlerSignatureDiagnostic(fileSet, function))
				continue
			}
			route.RequestType = signature.RequestType
			route.ResponseType = signature.ResponseType
			controller.Routes = append(controller.Routes, *route)
		}
	}

	for i := range controllers {
		sortRoutes(controllers[i].Routes)
	}
	return controllers, diagnostics, nil
}

func parseControllersFromAST(pkg Package, file string, fileSet *token.FileSet, parsed *ast.File, diagnoseFuncDecls bool) ([]Controller, []Diagnostic) {
	controllers := make([]Controller, 0)
	diagnostics := make([]Diagnostic, 0)
	for _, declaration := range parsed.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok {
			if diagnoseFuncDecls {
				diagnostics = append(diagnostics, invalidTargetDiagnosticsForDeclaration(fileSet, declaration)...)
			}
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

		imports := importAliases(parsed)
		var controller *Controller
		var tag string
		hidden := false
		guards := []GuardReference{}
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
					Hidden:   hidden,
					Guards:   guards,
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
			case "Hide":
				if !parseNoArguments(decorator.Raw) {
					diagnostics = append(diagnostics, invalidSyntaxDiagnostic(
						decorator,
						"@Hide requires no arguments",
						"use @Hide()",
					))
					continue
				}
				hidden = true
				if controller == nil {
					controller = &Controller{
						Package:  pkg,
						TypeName: typeName,
						File:     file,
					}
				}
				controller.Hidden = true
			case "Use":
				guard, ok := parseUseGuard(decorator, imports, &diagnostics)
				if ok {
					guards = append(guards, guard)
					if controller == nil {
						controller = &Controller{
							Package:  pkg,
							TypeName: typeName,
							File:     file,
						}
					}
					controller.Guards = append(controller.Guards, guard)
				}
			default:
				diagnostics = append(diagnostics, unknownDecoratorDiagnostic(decorator))
			}
		}
		if controller != nil && controller.BasePath != "" {
			controllers = append(controllers, *controller)
		}
	}
	return controllers, diagnostics
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

func parseRouteDecorators(handlerName string, decorators []decorator, imports map[string]string) (*Route, []Diagnostic) {
	route := &Route{
		HandlerName: handlerName,
		Statuses:    make([]int, 0),
	}
	diagnostics := make([]Diagnostic, 0)
	var methodDecorator *decorator
	hasRoute := false

	for _, decorator := range decorators {
		switch {
		case isHTTPMethodDecorator(decorator.Name):
			if methodDecorator != nil {
				diagnostics = append(diagnostics, invalidSyntaxDiagnostic(
					decorator,
					"route method has multiple HTTP method decorators",
					"use exactly one of @Get, @Post, @Put, @Patch, or @Delete",
				))
				continue
			}
			path, ok := parseSingleStringArgument(decorator.Raw)
			if !ok {
				diagnostics = append(diagnostics, invalidSyntaxDiagnostic(
					decorator,
					"@"+decorator.Name+" requires a single string path argument",
					`use @`+decorator.Name+`("/path")`,
				))
				continue
			}
			if !strings.HasPrefix(path, "/") {
				diagnostics = append(diagnostics, invalidSyntaxDiagnostic(
					decorator,
					"@"+decorator.Name+" path must start with /",
					`use @`+decorator.Name+`("/path")`,
				))
				continue
			}
			methodDecorator = &decorator
			hasRoute = true
			route.Method = strings.ToUpper(decorator.Name)
			route.Path = path
			route.File = decorator.File
			route.Line = decorator.Line
			route.Column = decorator.Column
		case decorator.Name == "Status":
			status, ok := parseSingleIntArgument(decorator.Raw)
			if !ok {
				diagnostics = append(diagnostics, invalidSyntaxDiagnostic(
					decorator,
					"@Status requires a single integer argument",
					"use @Status(200)",
				))
				continue
			}
			route.Statuses = append(route.Statuses, status)
			hasRoute = true
		case decorator.Name == "Summary":
			summary, ok := parseSingleStringArgument(decorator.Raw)
			if !ok {
				diagnostics = append(diagnostics, invalidSyntaxDiagnostic(
					decorator,
					"@Summary requires a single string argument",
					`use @Summary("Find user")`,
				))
				continue
			}
			route.Summary = summary
			hasRoute = true
		case decorator.Name == "Description":
			description, ok := parseSingleStringArgument(decorator.Raw)
			if !ok {
				diagnostics = append(diagnostics, invalidSyntaxDiagnostic(
					decorator,
					"@Description requires a single string argument",
					`use @Description("Returns a user by ID")`,
				))
				continue
			}
			route.Description = description
			hasRoute = true
		case decorator.Name == "Hide":
			if !parseNoArguments(decorator.Raw) {
				diagnostics = append(diagnostics, invalidSyntaxDiagnostic(
					decorator,
					"@Hide requires no arguments",
					"use @Hide()",
				))
				continue
			}
			route.Hidden = true
			hasRoute = true
		case decorator.Name == "Use":
			guard, ok := parseUseGuard(decorator, imports, &diagnostics)
			if ok {
				route.Guards = append(route.Guards, guard)
				hasRoute = true
			}
		case isControllerDecorator(decorator.Name):
			continue
		default:
			if isDeferredDecorator(decorator.Name) || strings.HasPrefix(decorator.Raw, "@") {
				diagnostics = append(diagnostics, unknownRouteDecoratorDiagnostic(decorator))
			}
		}
	}

	if !hasRoute || route.Method == "" {
		return nil, diagnostics
	}
	return route, diagnostics
}

func parseUseGuard(decorator decorator, imports map[string]string, diagnostics *[]Diagnostic) (GuardReference, bool) {
	alias, symbol, ok := parseSingleSelectorArgument(decorator.Raw)
	if !ok {
		*diagnostics = append(*diagnostics, invalidSyntaxDiagnostic(
			decorator,
			"@Use requires a single imported middleware or guard selector argument",
			"use @Use(auth.JWTGuard) or @Use(requestlog.Audit)",
		))
		return GuardReference{}, false
	}
	importPath, ok := imports[alias]
	if !ok {
		*diagnostics = append(*diagnostics, unresolvedGuardAliasDiagnostic(decorator, alias))
		return GuardReference{}, false
	}
	return GuardReference{
		Alias:      alias,
		Symbol:     symbol,
		ImportPath: importPath,
		File:       decorator.File,
		Line:       decorator.Line,
		Column:     decorator.Column,
	}, true
}

type decorator struct {
	Name   string
	Raw    string
	File   string
	Line   int
	Column int
}

type handlerSignature struct {
	RequestType  string
	ResponseType string
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

func parseSingleIntArgument(raw string) (int, bool) {
	open := strings.Index(raw, "(")
	close := strings.LastIndex(raw, ")")
	if open == -1 || close != len(raw)-1 || close <= open {
		return 0, false
	}
	argument := strings.TrimSpace(raw[open+1 : close])
	value, err := strconv.Atoi(argument)
	if err != nil {
		return 0, false
	}
	return value, true
}

func parseSingleSelectorArgument(raw string) (string, string, bool) {
	open := strings.Index(raw, "(")
	close := strings.LastIndex(raw, ")")
	if open == -1 || close != len(raw)-1 || close <= open {
		return "", "", false
	}
	argument := strings.TrimSpace(raw[open+1 : close])
	parts := strings.Split(argument, ".")
	if len(parts) != 2 || !isIdentifier(parts[0]) || !isIdentifier(parts[1]) {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func parseNoArguments(raw string) bool {
	open := strings.Index(raw, "(")
	close := strings.LastIndex(raw, ")")
	if open == -1 || close != len(raw)-1 || close <= open {
		return false
	}
	return strings.TrimSpace(raw[open+1:close]) == ""
}

func isIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for index, r := range value {
		if index == 0 {
			if r != '_' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
				return false
			}
			continue
		}
		if r != '_' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

func importAliases(file *ast.File) map[string]string {
	imports := make(map[string]string)
	for _, spec := range file.Imports {
		if spec.Path == nil {
			continue
		}
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		if spec.Name != nil {
			if spec.Name.Name == "." || spec.Name.Name == "_" {
				continue
			}
			imports[spec.Name.Name] = importPath
			continue
		}
		imports[defaultImportAlias(importPath)] = importPath
	}
	return imports
}

func defaultImportAlias(importPath string) string {
	base := filepath.Base(importPath)
	return strings.TrimSuffix(base, ".go")
}

func validateHandlerSignature(function *ast.FuncDecl) (handlerSignature, bool) {
	params := function.Type.Params
	results := function.Type.Results
	if params == nil || len(params.List) == 0 || fieldCount(params.List) > 2 {
		return handlerSignature{}, false
	}

	flattenedParams := flattenFields(params.List)
	if len(flattenedParams) == 0 || len(flattenedParams) > 2 {
		return handlerSignature{}, false
	}
	if !isGestContextPointer(flattenedParams[0]) {
		return handlerSignature{}, false
	}

	signature := handlerSignature{}
	if len(flattenedParams) == 2 {
		requestType, ok := namedPointerType(flattenedParams[1])
		if !ok {
			return handlerSignature{}, false
		}
		signature.RequestType = requestType
	}

	flattenedResults := flattenResultFields(results)
	switch len(flattenedResults) {
	case 1:
		if !isErrorType(flattenedResults[0]) {
			return handlerSignature{}, false
		}
		return signature, true
	case 2:
		responseType, ok := namedPointerType(flattenedResults[0])
		if !ok || !isErrorType(flattenedResults[1]) {
			return handlerSignature{}, false
		}
		signature.ResponseType = responseType
		return signature, true
	default:
		return handlerSignature{}, false
	}
}

func fieldCount(fields []*ast.Field) int {
	count := 0
	for _, field := range fields {
		if len(field.Names) == 0 {
			count++
			continue
		}
		count += len(field.Names)
	}
	return count
}

func flattenFields(fields []*ast.Field) []ast.Expr {
	expressions := make([]ast.Expr, 0, fieldCount(fields))
	for _, field := range fields {
		count := len(field.Names)
		if count == 0 {
			count = 1
		}
		for range count {
			expressions = append(expressions, field.Type)
		}
	}
	return expressions
}

func flattenResultFields(results *ast.FieldList) []ast.Expr {
	if results == nil {
		return nil
	}
	return flattenFields(results.List)
}

func isGestContextPointer(expression ast.Expr) bool {
	pointer, ok := expression.(*ast.StarExpr)
	if !ok {
		return false
	}
	selector, ok := pointer.X.(*ast.SelectorExpr)
	if !ok || selector.Sel == nil || selector.Sel.Name != "Context" {
		return false
	}
	packageName, ok := selector.X.(*ast.Ident)
	return ok && packageName.Name == "gest"
}

func namedPointerType(expression ast.Expr) (string, bool) {
	pointer, ok := expression.(*ast.StarExpr)
	if !ok {
		return "", false
	}
	switch typed := pointer.X.(type) {
	case *ast.Ident:
		return typed.Name, true
	case *ast.SelectorExpr:
		if typed.Sel != nil {
			return typed.Sel.Name, true
		}
	}
	return "", false
}

func isErrorType(expression ast.Expr) bool {
	ident, ok := expression.(*ast.Ident)
	return ok && ident.Name == "error"
}

func invalidHandlerSignatureDiagnostic(fileSet *token.FileSet, function *ast.FuncDecl) Diagnostic {
	position := fileSet.Position(function.Pos())
	return Diagnostic{
		Severity: SeverityError,
		Code:     DiagnosticInvalidHandlerSignature,
		Message:  "invalid handler signature " + handlerSignatureString(function),
		Hint:     "accepted signatures are func(ctx *gest.Context) error, func(ctx *gest.Context) (*Res, error), func(ctx *gest.Context, req *Req) (*Res, error), and func(ctx *gest.Context, req *Req) error",
		File:     position.Filename,
		Line:     position.Line,
		Column:   position.Column,
		Target:   function.Name.Name,
	}
}

func handlerSignatureString(function *ast.FuncDecl) string {
	var params string
	if function.Type.Params != nil {
		params = exprListString(flattenFields(function.Type.Params.List))
	}
	results := exprListString(flattenResultFields(function.Type.Results))
	if results == "" {
		return "func(" + params + ")"
	}
	if strings.Contains(results, ", ") {
		results = "(" + results + ")"
	}
	return "func(" + params + ") " + results
}

func exprListString(expressions []ast.Expr) string {
	parts := make([]string, 0, len(expressions))
	for _, expression := range expressions {
		parts = append(parts, exprString(expression))
	}
	return strings.Join(parts, ", ")
}

func exprString(expression ast.Expr) string {
	switch typed := expression.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.StarExpr:
		return "*" + exprString(typed.X)
	case *ast.SelectorExpr:
		return exprString(typed.X) + "." + typed.Sel.Name
	default:
		return fmt.Sprintf("%T", expression)
	}
}

func isControllerDecorator(name string) bool {
	return name == "Controller" || name == "Tag" || name == "Hide" || name == "Use"
}

func isHTTPMethodDecorator(name string) bool {
	switch name {
	case "Get", "Post", "Put", "Patch", "Delete":
		return true
	default:
		return false
	}
}

func isRouteMetadataDecorator(name string) bool {
	switch name {
	case "Status", "Summary", "Description", "Hide":
		return true
	default:
		return false
	}
}

func isRouteRelevantDecorator(name string) bool {
	return isHTTPMethodDecorator(name) || isRouteMetadataDecorator(name) || name == "Use" || isDeferredDecorator(name)
}

func hasRouteRelevantDecorator(decorators []decorator) bool {
	for _, decorator := range decorators {
		if isRouteRelevantDecorator(decorator.Name) {
			return true
		}
	}
	return false
}

func isDeferredDecorator(name string) bool {
	switch name {
	case "Auth", "Public", "Roles", "Permissions", "Cache", "Throttle", "Stream", "WebSocket", "Processor", "Cron":
		return true
	default:
		return false
	}
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
		Hint:     "supported MVP decorators are @Controller, @Tag, @Hide, @Use, @Get, @Post, @Put, @Patch, @Delete, @Status, @Summary, and @Description",
		File:     decorator.File,
		Line:     decorator.Line,
		Column:   decorator.Column,
		Target:   decorator.Name,
	}
}

func unknownRouteDecoratorDiagnostic(decorator decorator) Diagnostic {
	return Diagnostic{
		Severity: SeverityError,
		Code:     DiagnosticUnknownDecorator,
		Message:  "unknown or deferred route decorator @" + decorator.Name,
		Hint:     "supported MVP route decorators are @Hide, @Use, @Get, @Post, @Put, @Patch, @Delete, @Status, @Summary, and @Description",
		File:     decorator.File,
		Line:     decorator.Line,
		Column:   decorator.Column,
		Target:   decorator.Name,
	}
}

func unresolvedGuardAliasDiagnostic(decorator decorator, alias string) Diagnostic {
	return Diagnostic{
		Severity: SeverityError,
		Code:     DiagnosticInvalidDecoratorSyntax,
		Message:  "unresolved middleware or guard import alias " + alias,
		Hint:     "import the middleware or guard package in this controller file; package scanning and hidden registries are not used",
		File:     decorator.File,
		Line:     decorator.Line,
		Column:   decorator.Column,
		Target:   decorator.Name,
	}
}

func invalidRouteTargetDiagnostic(decorator decorator, target string) Diagnostic {
	if target == "" {
		target = "<declaration>"
	}
	return Diagnostic{
		Severity: SeverityError,
		Code:     DiagnosticInvalidTarget,
		Message:  "@" + decorator.Name + " can only apply to methods on parsed controller types",
		Hint:     "move the route decorator to a method on a type with @Controller",
		File:     decorator.File,
		Line:     decorator.Line,
		Column:   decorator.Column,
		Target:   target,
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

func sortRoutes(routes []Route) {
	slices.SortFunc(routes, func(a Route, b Route) int {
		if filepath.ToSlash(a.File) < filepath.ToSlash(b.File) {
			return -1
		}
		if filepath.ToSlash(a.File) > filepath.ToSlash(b.File) {
			return 1
		}
		if a.Line < b.Line {
			return -1
		}
		if a.Line > b.Line {
			return 1
		}
		if a.HandlerName < b.HandlerName {
			return -1
		}
		if a.HandlerName > b.HandlerName {
			return 1
		}
		return 0
	})
}

func receiverTypeName(function *ast.FuncDecl) string {
	if function.Recv == nil || len(function.Recv.List) != 1 {
		return ""
	}
	switch receiver := function.Recv.List[0].Type.(type) {
	case *ast.Ident:
		return receiver.Name
	case *ast.StarExpr:
		if ident, ok := receiver.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return ""
}
