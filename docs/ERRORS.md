# Gest Error Model

This document is the implementation contract for runtime errors and generator diagnostics. Later implementation tasks must treat these names, categories, fields, and mappings as stable test targets.

## Principles

- Expected user mistakes return errors, not panics.
- Framework errors must be structured enough for tests to assert category, code, message, context, and HTTP status.
- Runtime errors and generator diagnostics are related but separate models.
- HTTP errors must map cleanly to status codes.
- Generator errors must include file and line when that information is available.
- Error messages must be actionable and include hints when an obvious fix exists.
- Internal implementation details may be logged, but user-facing messages must not require reading framework source code.

## Runtime Error Shape

Runtime and bootstrap failures use a structured framework error with these logical fields:

| Field | Required | Description |
| --- | --- | --- |
| `kind` | Yes | Broad category such as `BadRequest`, `Conflict`, or `Internal`. |
| `code` | Yes | Stable machine-readable code such as `DI_MISSING_PROVIDER`. |
| `message` | Yes | Short human-readable explanation of what failed. |
| `hint` | No | Actionable remediation when the fix is obvious. |
| `module` | When available | Module name involved in the failure. |
| `provider` | When available | Provider token, constructor, or exported name involved in the failure. |
| `controller` | When available | Controller type or metadata name involved in the failure. |
| `route` | When available | Route method and path, formatted consistently as `METHOD /path`. |
| `field` | When available | Request binding field, parameter, query key, header key, or body path. |
| `cause` | No | Wrapped lower-level error for programmatic inspection and logs. |

The public API may expose this as a Go struct, interface, or concrete error type, but tests must be able to inspect `kind`, `code`, `message`, `hint`, and relevant context without parsing `Error()` text.

`Error()` should produce concise text in this shape:

```text
DI_MISSING_PROVIDER: missing provider for token UserService in module ReportsModule. Hint: add a provider or import a module that exports UserService.
```

## HTTP Runtime Errors

HTTP handlers and adapters must map framework error kinds to status codes as follows:

| Kind | Status | Use |
| --- | ---: | --- |
| `BadRequest` | 400 | Malformed requests, binding failures, conversion failures, validation failures, invalid runtime input. |
| `Unauthorized` | 401 | Missing or invalid authentication credentials. |
| `Forbidden` | 403 | Authenticated request lacks permission for the resource or action. |
| `NotFound` | 404 | Requested route or resource is not found. |
| `Conflict` | 409 | Duplicate runtime definitions or state conflicts detected before serving a request. |
| `Internal` | 500 | Framework bugs, unexpected failures, and intentionally hidden implementation details. |

HTTP error responses must be deterministic JSON by default:

```json
{
  "error": {
    "kind": "BadRequest",
    "code": "BINDING_CONVERSION_FAILURE",
    "message": "query field limit must be an integer",
    "hint": "Use a base-10 integer value for ?limit=.",
    "field": "query.limit"
  }
}
```

Adapters may allow customization later, but the default mapping and fields are the test contract.

## DI Errors

DI errors occur while building or resolving the application graph. They must never panic for ordinary user configuration mistakes.

| Code | Kind | Required Context | Message Contract |
| --- | --- | --- | --- |
| `DI_MISSING_PROVIDER` | `Internal` during bootstrap, `BadRequest` only if exposed through an explicit runtime lookup | `module`, `provider` | State the missing token/provider and where it was requested. Hint to add a provider or import a module that exports it. |
| `DI_PROVIDER_CYCLE` | `Internal` | `module`, provider path | Include the full cycle path in order. Hint to split responsibilities or inject an interface/value that breaks the cycle. |
| `DI_DUPLICATE_PROVIDER` | `Conflict` | `module`, `provider` | Name the duplicate provider token and module. Hint to remove one provider or use a distinct token/name. |
| `DI_UNEXPORTED_PROVIDER` | `Internal` | importing `module`, exporting module, `provider` | Explain that the provider exists in another module but is not exported. Hint to export it or provide it locally. |
| `DI_UNSUPPORTED_SCOPE` | `BadRequest` | `module`, `provider` | Name the unsupported scope. Hint that the MVP supports singleton scope only. |

## Module Errors

Module errors occur while validating the module graph.

| Code | Kind | Required Context | Message Contract |
| --- | --- | --- | --- |
| `MODULE_DUPLICATE` | `Conflict` | `module` | Name the duplicated module. Hint to import it once or use a distinct module name. |
| `MODULE_INVALID_IMPORT` | `BadRequest` | importing `module` and invalid import | Explain why the import is invalid, nil, cyclic at the module layer, or not a module. |
| `MODULE_EXPORT_NOT_FOUND` | `Internal` | `module`, export token/name | Name the missing export. Hint to add it to the module providers before exporting it. |

## Route Errors

Route errors occur while validating or registering controller route metadata.

| Code | Kind | Required Context | Message Contract |
| --- | --- | --- | --- |
| `ROUTE_DUPLICATE` | `Conflict` | `controller`, `route` | Name the duplicate route method/path and both controllers when available. |
| `ROUTE_INVALID_METHOD` | `BadRequest` | `controller`, route method | Name the invalid HTTP method. Hint to use a supported method such as `GET`, `POST`, `PUT`, `PATCH`, or `DELETE`. |
| `ROUTE_INVALID_PATH` | `BadRequest` | `controller`, route path | Explain the path problem. Hint to use a leading slash and valid path parameters. |
| `ROUTE_MISSING_CONTROLLER_METADATA` | `Internal` | `controller` | State that controller metadata is missing. Hint to add hand-written `GestController()` metadata or run the generator. |

## Handler Errors

Handler errors occur while validating route handlers before serving traffic.

| Code | Kind | Required Context | Message Contract |
| --- | --- | --- | --- |
| `HANDLER_INVALID_SIGNATURE` | `BadRequest` | `controller`, `route` | Describe the received signature and accepted MVP signatures. |
| `HANDLER_UNSUPPORTED_RETURN_TYPE` | `BadRequest` | `controller`, `route` | Name the unsupported return type. Hint to return `error`, `(*Res, error)`, or another explicitly supported runtime type. |

## Binding Errors

Binding errors occur while reading request params, query values, headers, and JSON body fields.

| Code | Kind | Required Context | Message Contract |
| --- | --- | --- | --- |
| `BINDING_MISSING_REQUIRED_PARAM` | `BadRequest` | `route`, `field` | Name the missing path parameter. |
| `BINDING_MISSING_REQUIRED_QUERY` | `BadRequest` | `route`, `field` | Name the missing query key. Hint to add `?key=value` when obvious. |
| `BINDING_MISSING_REQUIRED_HEADER` | `BadRequest` | `route`, `field` | Name the missing header. |
| `BINDING_MISSING_REQUIRED_BODY_FIELD` | `BadRequest` | `route`, `field` | Name the missing JSON field path. |
| `BINDING_CONVERSION_FAILURE` | `BadRequest` | `route`, `field` | Name the received value and target type when safe. Hint with the expected format when obvious. |
| `BINDING_VALIDATION_FAILURE` | `BadRequest` | `route`, `field` when available | Include the validation rule or validator message. Hint with the expected constraint when obvious. |

Binding errors returned to HTTP clients must not expose Go reflection internals. Field names should use request-facing names from route params, query tags, header tags, and JSON tags.

## Generator Diagnostic Shape

Generator diagnostics are not runtime errors. They describe source problems and generated-file failures before the program runs.

Each diagnostic has these logical fields:

| Field | Required | Description |
| --- | --- | --- |
| `severity` | Yes | `error` for generation-blocking diagnostics. `warning` may be added later, but MVP diagnostics are errors. |
| `code` | Yes | Stable machine-readable code such as `GEN_UNKNOWN_DECORATOR`. |
| `message` | Yes | Human-readable explanation of the source or filesystem problem. |
| `hint` | No | Actionable remediation when obvious. |
| `file` | When available | Source file or generated file path. |
| `line` | When available | One-based line number. |
| `column` | No | One-based column number if parser support makes it cheap and reliable. |
| `target` | When available | Decorated type, method, import, or output path. |

CLI output must be copy-paste useful and should include location before the message when file/line are available:

```text
internal/users/controller.go:17: GEN_INVALID_DECORATOR_SYNTAX: @Get requires a string path argument. Hint: use @Get("/users/:id").
```

## Generator Errors

| Code | Required Context | Message Contract |
| --- | --- | --- |
| `GEN_UNKNOWN_DECORATOR` | `file`, `line`, decorator name | Name the unsupported decorator. Hint with the supported MVP decorators when useful. |
| `GEN_INVALID_DECORATOR_SYNTAX` | `file`, `line`, decorator name | Explain the expected syntax. Hint with a concrete valid example. |
| `GEN_INVALID_TARGET` | `file`, `line`, target | Explain why the decorator cannot apply to that target. Hint with the allowed target type. |
| `GEN_AMBIGUOUS_IMPORT` | `file`, `line` when available, import path or symbol | Name the ambiguous import/symbol and the candidates. Hint to use an explicit import alias or remove the ambiguity. |
| `GEN_WRITE_FAILURE` | generated file path, wrapped cause | Explain which generated file could not be written. Include the filesystem cause in logs and a concise user-facing message. |

Generator diagnostics must be deterministic in order. Sort by file path, then line, then code unless a later implementation documents a more precise source-order rule.

## Test Requirements

Implementation tasks that introduce these errors must add tests that assert structured fields directly. At minimum:

- HTTP mapping tests assert status code and default JSON shape.
- DI and module tests assert `code`, `kind`, `module`, `provider`, and actionable hints.
- Route and handler tests assert duplicate route and invalid signature details.
- Binding tests assert request-facing field names and 400 responses.
- Generator tests assert diagnostic code, file, line, message, hint, and deterministic ordering.
