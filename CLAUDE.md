# cue-user-funcs Contributor Guidelines

## Project overview

A Go program that emulates `cue export` with user-provided semver functions.
Uses a fork of `cuelang.org/go` (`github.com/myitcvforks/cue`, branch
`user_funcs_etc`) that implements WIP user-provided functions and value
injection.

## Key workflows

### Build and run

```bash
# Run the export command
go run . export ./testdata

# Run tests
go test ./...
```

### Testing

Use `tmp/` (gitignored) for temporary artifacts. When creating temporary Go
programs in `tmp/`, each needs its own `go.mod` to prevent interference with
the main module's `./...` pattern matching.

CUE test data goes in `testdata/`.

### Shell commands

Always use `command cd` when changing directories in shell scripts, as plain
`cd` may be overridden by shell functions. For all other commands, use plain
names without the `command` prefix.

## Project structure

- `main.go` - Entry point. Registers semver functions via the injector API,
  loads CUE packages, outputs JSON.
- `testdata/` - Testscript txtar files.
- `go.mod` - Module definition with replace directive pointing to the CUE fork.

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

## Rules

- Do not set `GONOSUMCHECK` or `GONOSUMDB` environment variables.
- Injected functions use hidden definitions (e.g. `#semverIsValid`) in CUE.
- Commit messages: subject line under 50 characters, body explaining the "why."
- Every commit must pass `go test ./...`.
- Do not add Co-Authored-By trailers to commit messages.
