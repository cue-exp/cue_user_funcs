# cue_user_funcs Contributor Guidelines

## Project overview

A Go program that emulates `cue export` with user-provided functions. It
dynamically discovers `@inject` attributes in CUE code, resolves backing Go
functions from version-qualified package paths, generates a temporary Go module,
builds it, and execs it. Uses a fork of `cuelang.org/go`
(`github.com/cue-exp/cue`, branch `user_funcs_etc`) that implements WIP
user-provided functions and value injection.

This module is also a CUE module providing reusable CUE packages (`semver`,
`sprig`) that bind Go functions via `@extern(inject)` / `@inject` attributes.

## Key workflows

### Build and run

```bash
# Run the export command
go run . export <directory>

# Run tests
go test ./...
```

### CI

CI configuration is CUE source of truth in `internal/ci/`, generating
`.github/workflows/trybot.yaml`:

```bash
go generate ./internal/ci/...
```

### Testing

Use `tmp/` (gitignored) for temporary artifacts. When creating temporary Go
programs in `tmp/`, each needs its own `go.mod` to prevent interference with
the main module's `./...` pattern matching.

CUE test data goes in `testdata/` as testscript txtar files.

### Shell commands

Always use `command cd` when changing directories in shell scripts, as plain
`cd` may be overridden by shell functions. For all other commands, use plain
names without the `command` prefix.

## Project structure

- `main.go` - Entry point. Discovers @inject attrs, resolves function signatures
  from Go source via `go list -json` and `go/parser`, generates a temp Go module
  with typed wrappers, builds and execs it.
- `_template/main.go` - Embedded template for the generated program. Calls
  `registerAll(j)` defined in the generated `register.go`.
- `semver/semver.cue` - CUE package binding golang.org/x/mod/semver functions.
- `sprig/sprig.go` - Go implementations of sprig-compatible functions.
- `sprig/sprig.cue` - CUE package exposing sprig functions via @inject.
- `text/template/template.go` - Go NonZero function (text/template-style truthiness).
- `text/template/template.cue` - CUE package exposing NonZero via @inject.
- `testdata/` - Testscript txtar files.
- `internal/ci/` - CUE source of truth for CI workflows.
- `.github/workflows/` - Generated CI workflow YAML (do not edit directly).
- `cue.mod/module.cue` - CUE module declaration.
- `go.mod` - Go module with replace directive pointing to the CUE fork.

## Key APIs used

- `cue.PureFunc1` / `cue.PureFunc2` - Wrap Go functions as CUE-callable
  functions.
- `cuecontext.NewInjector` - Creates an injector for `@extern(inject)` /
  `@inject` attributes.
- `cue/load.Instances` - Loads CUE packages from disk.
- `cue.Context.BuildInstance` - Builds a loaded instance into a CUE value.

## Bug-fix process

1. Read the complete issue including all comments.
2. Reproduce the bug.
3. Reduce to a minimal failing test.
4. Commit the reproduction test separately.
5. Fix the bug in a second commit.
6. Cross-check against the original report.
7. Run the full test suite (`go test ./...`).

## Adding Go+CUE packages (self-referencing modules)

When adding a new Go+CUE package to this module (like `text/template`), the
`@inject` name embeds a Go module pseudo-version. Since the Go code must be
published before the inject name can resolve, use the `@test` mechanism to
test locally during development:

1. Create the Go source file (e.g. `text/template/template.go`).
2. Create the CUE binding file (e.g. `text/template/template.cue`) with a
   placeholder `@inject` pseudo-version. This file is the "production" binding
   that external consumers will use once a version is published.
3. Create a `_test.cue` file (e.g. `text/template/template_test.cue`) with:
   - `@if(test)` file-level attribute so it's only loaded with `--test`
   - `@inject(name="module@test/path.Func")` using `@test` as the version
   - Test expressions that exercise the function
4. Commit and push. Tests pass because the `_test.cue` exercises the local
   code via `@test`, and the production CUE file's stale pseudo-version is
   not tested.

After the commit is pushed and the Go pseudo-version is available:

5. Update the production CUE file's `@inject` pseudo-version to point to the
   newly published commit. The `_test.cue` file stays unchanged.
6. Add a testscript txtar file (e.g. `testdata/template.txtar`) that creates
   a standalone CUE module importing the package via its published version.
7. Update `testdata/import.txtar` to include usage of the new package.
8. Publish a new CUE module version so consumers can depend on it.

The `@test` version mechanism: when `cue_user_funcs` sees `@test` as the
version, it adds a `replace` directive pointing to the local module root.
When both `@test` and non-test inject names exist for the same function,
the `@test` version takes precedence and the non-test version is filtered out.
In `--test` mode, `cue.mod/inject.mod` and `inject.sum` are neither read
nor written back, avoiding pollution with local replace directives.
## Rules

- Do not set `GONOSUMCHECK` or `GONOSUMDB` environment variables.
- Injected functions use hidden definitions (e.g. `#semverIsValid`) in CUE.
- Commit messages: subject line under 50 characters, body explaining the "why."
- Always use `--no-gpg-sign` when creating commits.
- Every commit must pass `go test ./...` and `go tool staticcheck ./...`.
- Do not add Co-Authored-By trailers to commit messages.
- Do not edit generated files in `.github/workflows/` directly; edit the CUE
  source in `internal/ci/` and run `go generate ./internal/ci/...`.
