# cue_user_funcs

A Go program that emulates `cue export`, extended with user-provided functions.
It dynamically discovers `@inject` attributes in CUE code, resolves the backing
Go functions from version-qualified package paths, generates a temporary Go
module with typed wrappers, builds it, and execs it.

This uses CUE's WIP user-provided functions and value injection proposals
([#4293](https://github.com/cue-lang/proposal/blob/main/designs/4293-user-functions-and-validators.md),
[#4294](https://github.com/cue-lang/proposal/blob/main/designs/4294-value-injection.md))
via a [fork](https://github.com/cue-exp/cue/tree/user_funcs_etc) of
`cuelang.org/go`.

## Usage

```
go run . export [-shim] [-debug] <directory>
```

Flags:

- `-shim` — print the generated Go registration shim to stdout and exit
  (useful for inspecting or testing the generated code).
- `-debug` — print cache diagnostic messages to stderr.

The directory must contain a CUE package with `@extern(inject)` and `@inject`
attributes.

## How it works

The following diagram shows how `cue_user_funcs export` resolves inject
dependencies and produces JSON output:

```
                         CUE source files
                               |
                               v
                  ┌────────────────────────┐
                  │  1. Load CUE package   │
                  │     & walk transitive  │
                  │     imports            │
                  └───────────┬────────────┘
                              |
                              v
                  ┌────────────────────────┐
                  │  2. Collect @inject    │
                  │     names from files   │
                  │     with @extern       │
                  │     (inject)           │
                  └───────────┬────────────┘
                              |
                              v
                  ┌────────────────────────┐
                  │  3. Parse inject names │
                  │     into module,       │
                  │     version, import    │
                  │     path, func name    │
                  └───────────┬────────────┘
                              |
                              v
                  ┌────────────────────────┐
                  │  4. Check cache        │──── hit ──> exec cached binary
                  │     (shim + binary)    │
                  └───────────┬────────────┘
                          miss |
                              v
                  ┌────────────────────────┐
                  │  5. Create temp Go     │
                  │     module with        │
                  │     require directives │
                  │     for each           │
                  │     module@version     │
                  └───────────┬────────────┘
                              |
                              v
                  ┌────────────────────────┐
                  │  6. go mod tidy        │
                  │     (downloads         │
                  │     modules)           │
                  └───────────┬────────────┘
                              |
                              v
                  ┌────────────────────────┐
                  │  7. Load Go packages   │
                  │     via go/packages,   │
                  │     extract function   │
                  │     signatures from    │
                  │     parsed AST         │
                  └───────────┬────────────┘
                              |
                              v
                  ┌────────────────────────┐
                  │  8. Generate register  │
                  │     .go shim with      │
                  │     typed PureFunc     │
                  │     wrappers           │
                  └───────────┬────────────┘
                              |
                              v
                  ┌────────────────────────┐
                  │  9. go build           │
                  │     (compile the       │
                  │     generated module)  │
                  └───────────┬────────────┘
                              |
                              v
                  ┌────────────────────────┐
                  │ 10. Cache binary,      │
                  │     then syscall.Exec  │
                  │     the built program  │
                  └───────────┬────────────┘
                              |
                              v
                         JSON output
```

### Caching

A two-level content-addressed cache (using
`github.com/rogpeppe/go-internal/cache`) avoids redundant work:

- **Shim cache** — keyed on the sorted set of inject names. If the same set of
  functions is requested again, the generated `register.go` is served from
  cache without downloading modules or parsing Go source.
- **Binary cache** — keyed on the shim content, Go version, GOOS/GOARCH, and
  the embedded template. If the compiled binary is already cached, it is
  exec'd directly, skipping all code generation and compilation.

## CUE packages

This repository is both a Go module and a CUE module
(`github.com/cue-exp/cue_user_funcs`). It provides two reusable CUE packages
that consumers can import to get pre-wired `@inject` bindings without having to
write `@extern(inject)` / `@inject` attributes themselves.

A consuming CUE module declares this module as a dependency in
`cue.mod/module.cue` and imports the packages:

```cue
package example

import (
    "github.com/cue-exp/cue_user_funcs/semver"
    "github.com/cue-exp/cue_user_funcs/sprig"
)

valid: semver.#IsValid("v1.2.3")
snake: sprig.#Snakecase("HelloWorld")
```

### `semver` — pure CUE package

The `semver` package is a single CUE file (`semver/semver.cue`) that binds
directly to functions in [`golang.org/x/mod/semver`](https://pkg.go.dev/golang.org/x/mod/semver),
a third-party Go module. No Go code lives in this repository for semver — the
`@inject` attributes reference the upstream Go package by its module path and
version:

```cue
#IsValid: _ @inject(name="golang.org/x/mod@v0.33.0/semver.IsValid")
```

Functions: `#IsValid`, `#Compare`, `#Canonical`, `#Major`, `#MajorMinor`,
`#Prerelease`, `#Build`.

### `sprig` — Go+CUE package

The `sprig` package contains both Go source (`sprig/sprig.go`) and a CUE
binding file (`sprig/sprig.cue`). The Go file implements
[sprig](https://masterminds.github.io/sprig/)-compatible string and semver
functions using libraries like `github.com/Masterminds/goutils`,
`github.com/Masterminds/semver/v3`, and `github.com/huandu/xstrings`. The CUE
file then binds to these Go functions via `@inject` attributes.

Because the `@inject` names must reference a *published* Go module version, the
CUE file contains a pinned pseudo-version of this module itself:

```cue
#Snakecase: _ @inject(name="github.com/cue-exp/cue_user_funcs@v0.0.0-20260306200449-5ada224ec191/sprig.Snakecase")
```

This creates a two-step publish ordering:

1. **Publish the Go module first** — push a commit containing the Go code in
   `sprig/sprig.go` so that a pseudo-version (or tag) becomes available on the
   Go module proxy.
2. **Update and publish the CUE module** — update `sprig/sprig.cue` to
   reference the newly published Go version in the `@inject` names, then tag
   and publish the CUE module.

Functions: `#Untitle`, `#Substr`, `#Nospace`, `#Trunc`, `#Abbrev`,
`#Abbrevboth`, `#Initials`, `#Wrap`, `#WrapWith`, `#Indent`, `#Nindent`,
`#Snakecase`, `#Camelcase`, `#Kebabcase`, `#Swapcase`, `#Plural`,
`#SemverCompare`, `#Semver`.

## Inject name format

Inject names are version-qualified Go package paths:

```
module@version/subpath.FuncName
```

For example:
- `golang.org/x/mod@v0.33.0/semver.IsValid`
- `github.com/cue-exp/cue_user_funcs@v0.0.0-20260306200449-5ada224ec191/sprig.Snakecase`

## CUE package setup

CUE files that use injected functions directly must have `@extern(inject)` at
the file level and `@inject(name=...)` on fields:

```cue
@extern(inject)

package mypackage

#semverIsValid: _ @inject(name="golang.org/x/mod@v0.33.0/semver.IsValid")

result: #semverIsValid("v1.0.0")
```

Alternatively, import the provided CUE packages which handle the wiring:

```cue
package mypackage

import "github.com/cue-exp/cue_user_funcs/semver"

result: semver.#IsValid("v1.0.0")
```

## CI

CI configuration lives in `internal/ci/` as CUE source of truth, generating
`.github/workflows/trybot.yaml` via:

```
go generate ./internal/ci/...
```

## Checksum verification (`inject.sum`)

When running inside a CUE module, `cue_user_funcs` maintains a
`cue.mod/inject.sum` file that pins the cryptographic hashes of downloaded Go
modules — the same `h1:` hashes that Go uses in `go.sum`.

On the first run, `inject.sum` is created automatically. On subsequent runs,
the file is copied into the temporary build directory as `go.sum` before
`go mod tidy` and `go build`, so the Go toolchain itself verifies that
downloaded modules match the recorded hashes. If a hash doesn't match, the
build fails with Go's standard `SECURITY ERROR`.

After a successful build, the resulting `go.sum` is written back to
`cue.mod/inject.sum`, capturing any new or updated entries. This file should be
checked into version control so that reviewers can see when dependency hashes
change.
