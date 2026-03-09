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

### `export`

```
go run . export [-shim] [-debug] [-test] <directory>
```

Flags:

- `-shim` — print the generated Go registration shim to stdout and exit
  (useful for inspecting or testing the generated code).
- `-debug` — print cache diagnostic messages to stderr.
- `-test` — include `@if(test)` guarded CUE files and `_test.cue` files in
  the load. `@test` versioned inject names resolve via local replace
  directives (see [Local development with `@test`](#local-development-with-test)).

The directory must contain a CUE package with `@extern(inject)` and `@inject`
attributes.

### `mod tidy`

```
go run . mod tidy
```

Runs `cue mod tidy`, then walks all CUE packages in the module to discover
`@inject` attributes, creates a temporary Go module with the required
dependencies, runs `go mod tidy`, and writes the resulting `go.mod` and `go.sum`
back to `cue.mod/inject.mod` and `cue.mod/inject.sum`.

**Note:** Ideally `mod tidy` would walk all `.cue` files and their transitive
imports the same way `cue mod tidy` does (direct filesystem walk via
`modimports.AllModuleFiles`, skipping `_`/`.`-prefixed entries and nested
modules). Currently it uses `cue/load` with the `./...` pattern as a simpler
approximation. In practice both approaches discover the same `@inject`
attributes, since we only care about loadable CUE packages.

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
(`github.com/cue-exp/cue_user_funcs`). It provides reusable CUE packages
that consumers can import to get pre-wired `@inject` bindings without having to
write `@extern(inject)` / `@inject` attributes themselves.

A consuming CUE module declares this module as a dependency in
`cue.mod/module.cue` and imports the packages:

```cue
package example

import (
    "github.com/cue-exp/cue_user_funcs/semver"
    "github.com/cue-exp/cue_user_funcs/sprig"
    "github.com/cue-exp/cue_user_funcs/net/url"
    "github.com/cue-exp/cue_user_funcs/text/template"
)

valid:   semver.#IsValid("v1.2.3")
snake:   sprig.#Snakecase("HelloWorld")
parsed:  url.#Parse("https://example.com/path?q=1")
nonZero: template.#NonZero("hello")
```

There are three kinds of CUE packages in this module, distinguished by where
the backing Go code lives:

### `semver` — pure CUE package (third-party Go)

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

### `net/url` — pure CUE package (stdlib)

The `net/url` package is a CUE binding file (`net/url/url.cue`) and a test file
(`net/url/url_test.cue`) that bind directly to Go's standard library
[`net/url.Parse`](https://pkg.go.dev/net/url#Parse). Like `semver`, no Go code
is needed — but unlike third-party modules, stdlib
packages don't require a `go mod edit -require` directive. The inject name
omits the version entirely:

```cue
#Parse: _ @inject(name="net/url.Parse")
```

Standard library packages are detected by the absence of a dot in the first
path element (e.g. `net/url` vs `golang.org/x/mod`), and their require
directives are skipped automatically.

Functions: `#Parse`.

### `text/template` — Go+CUE package

The `text/template` package contains Go source (`text/template/template.go`),
a CUE binding file (`text/template/template.cue`), and a test file
(`text/template/template_test.cue`). The Go file implements a `NonZero`
function that follows the Go standard library's `text/template` documentation
for `{{if}}` actions: empty values are `false`, `0`, any nil pointer or
interface value, any array/slice/map/string of length zero, and any zero-value
struct.

This differs from `text/template.IsTrue` which treats all structs as true
regardless of whether they are zero-valued
([golang/go#28394](https://github.com/golang/go/issues/28394)).

Functions: `#NonZero`.

### `sprig` — Go+CUE package

The `sprig` package contains Go source (`sprig/sprig.go`), a CUE binding file
(`sprig/sprig.cue`), and a test file (`sprig/sprig_test.cue`). The Go file
implements [sprig](https://masterminds.github.io/sprig/)-compatible string and
semver functions using libraries like `github.com/Masterminds/goutils`,
`github.com/Masterminds/semver/v3`, and `github.com/huandu/xstrings`. The CUE
file then binds to these Go functions via `@inject` attributes.

Because the `@inject` names must reference a *published* Go module version, the
CUE file contains a pinned pseudo-version of this module itself:

```cue
#Snakecase: _ @inject(name="github.com/cue-exp/cue_user_funcs@v0.0.0-20260306200449-5ada224ec191/sprig.Snakecase")
```

Functions: `#Untitle`, `#Substr`, `#Nospace`, `#Trunc`, `#Abbrev`,
`#Abbrevboth`, `#Initials`, `#Wrap`, `#WrapWith`, `#Indent`, `#Nindent`,
`#Snakecase`, `#Camelcase`, `#Kebabcase`, `#Swapcase`, `#Plural`,
`#SemverCompare`, `#Semver`.

### Go+CUE package publish ordering

Go+CUE packages in this module (currently `sprig` and `text/template`) have a
publish ordering requirement: the `@inject` name embeds a Go module
pseudo-version, and the Go code must be fetchable at that version before the
shim can resolve it.

To develop locally without waiting for publication, each Go+CUE package has a
`_test.cue` file (e.g. `sprig/sprig_test.cue`) that uses `@test` as the
version. When `cue_user_funcs export --test` is used, `@test` versions resolve
via local `replace` directives, bypassing the Go module proxy entirely.

The publish sequence for a new Go+CUE package:

1. **Initial commit** — add the Go source, the CUE binding file (with a
   placeholder pseudo-version), and the `_test.cue` file. The `_test.cue`
   file uses `@test` versions and `@if(test)` so it exercises the local code.
   All tests pass.
2. **After push** — the Go pseudo-version for commit 1 becomes available on
   the Go module proxy. Update the CUE binding file's `@inject` pseudo-version
   to the newly published commit. Add a testscript txtar test that uses the
   published version. Update `testdata/import.txtar` to include the new
   package.
3. **Publish CUE module** — tag and publish a new CUE module version so
   consumers can depend on it in their `cue.mod/module.cue`.

## Inject name format

Inject names are version-qualified Go package paths:

```
module@version/subpath.FuncName
```

For example:
- `golang.org/x/mod@v0.33.0/semver.IsValid` — third-party module
- `github.com/cue-exp/cue_user_funcs@v0.0.0-20260306200449-5ada224ec191/sprig.Snakecase` — this module's own Go code
- `net/url.Parse` — Go standard library (no version needed)
- `github.com/cue-exp/cue_user_funcs@test/text/template.NonZero` — local test (see below)

The version in the inject name is used as a minimum requirement in the
generated `go.mod`, but the actual version resolved by `go mod tidy` may be
higher due to Go's [MVS](https://research.swtch.com/vgo-mvs) algorithm — other
inject names, transitive dependencies, or any other constraint can cause a
newer version to be selected.

### `@test` version for local development

The special version `test` resolves against the local module source instead of
the Go module proxy. When `cue_user_funcs export --test` is used:

1. CUE files guarded with `@if(test)` and files with the `_test.cue` suffix
   are included in the load.
2. `@inject` names with `@test` take precedence over non-test versions of the
   same function — the non-test versions are filtered out entirely.
3. The generated Go module uses a `replace` directive pointing to the local
   module root, so it builds against the working tree.
4. `cue.mod/inject.mod` and `cue.mod/inject.sum` are neither read nor written,
   avoiding pollution with local replace directives.

Each CUE package has a `_test.cue` file (e.g. `sprig/sprig_test.cue`,
`text/template/template_test.cue`, `net/url/url_test.cue`) that defines
inject bindings plus test expressions. Go+CUE packages use `@test` versions;
stdlib packages like `net/url` use the same inject name since they don't
require the Go module proxy. This allows local testing in CI before a
pseudo-version is published.

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
