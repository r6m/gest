# Contributing To Gest

This project is intentionally strict. Gest is a framework, so small API choices become long-term user contracts.

## Required Reading

Before implementing a task, read:

1. `AGENTS.md`
2. `.skills/RTK.md`
3. `docs/TASKS.md`
4. `docs/ARCHITECTURE.md`
5. `docs/DECISIONS.md`

## Command Rules

Project instructions require shell commands to be prefixed with `rtk`.

Use:

```bash
rtk go test ./...
rtk proxy golangci-lint run ./...
```

Use `rtk proxy` for `golangci-lint` because the plain `rtk golangci-lint` wrapper may add flags that are not supported by the installed version.

## Definition Of Done

A task is done only when all of these are true:

- The implementation stays scoped to the assigned task.
- Deferred features are not implemented.
- Tests are added or updated unless the task is documentation-only.
- `rtk go test ./...` passes.
- `rtk proxy golangci-lint run ./...` passes.
- Generated code, if any, is deterministic and gofmt/gofumpt-compatible.
- `docs/TASKS.md` status is updated only for tasks that are genuinely complete.
- Any blocker is documented with exact command output or a concrete missing prerequisite.

## Testing Expectations

Runtime tasks need tests for success and failure paths.

Generator tasks need fixture tests for:

- valid decorators
- invalid decorators
- invalid handler signatures
- deterministic generated output
- golden generated output

CLI tasks need command tests for:

- success exit codes
- failure exit codes
- config defaults
- output clarity
- visible underlying Go commands

Optional modules need:

- unit tests for module behavior
- integration-style usage tests when they expose runtime behavior
- tests proving user modules can replace official modules

## Lint Expectations

The root `.golangci.yml` is the lint contract.

Do not add blanket lint disables. Fix the code. If a narrow exception is unavoidable, document the reason near the exception and mention it in the task result.

## Scope Control

Do not implement deferred features while working on earlier phases.

Deferred until explicitly assigned:

- Fiber adapter
- built-in database module
- ORM abstraction
- lazy modules
- request scope
- transient scope
- runtime source scanning
- hidden route registries
- `init()` route registration
- OpenAPI before typed handlers are stable
- `gest dev` before `gest generate` and `gest build` are reliable

## Public API Review

Any new exported type, function, method, interface, package, or module path must be reviewed against:

- Go naming conventions
- compatibility risk
- whether the name will still make sense after later phases
- whether the same behavior can stay internal for now

Prefer fewer exported APIs until the runtime shape is proven.

## Updating `docs/TASKS.md`

Only mark a task `Done` when the definition of done is satisfied.

Use `Blocked` only when a concrete prerequisite prevents progress. Include the prerequisite in the task result.

Do not mark a task done merely because part of it was implemented.
