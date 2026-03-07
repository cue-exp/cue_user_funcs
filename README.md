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

This module is also a CUE module (`github.com/cue-exp/cue_user_funcs`)
that provides reusable CUE packages:

### semver

Binds [`golang.org/x/mod/semver`](https://pkg.go.dev/golang.org/x/mod/semver)
functions: `#IsValid`, `#Compare`, `#Canonical`, `#Major`, `#MajorMinor`,
`#Prerelease`, `#Build`.

```cue
import "github.com/cue-exp/cue_user_funcs/semver"

valid: semver.#IsValid("v1.2.3")
```

### sprig

Provides [sprig](https://masterminds.github.io/sprig/)-compatible string
functions: `#Untitle`, `#Substr`, `#Nospace`, `#Trunc`, `#Abbrev`,
`#Abbrevboth`, `#Initials`, `#Wrap`, `#WrapWith`, `#Indent`, `#Nindent`,
`#Snakecase`, `#Camelcase`, `#Kebabcase`, `#Swapcase`, `#Plural`,
`#SemverCompare`, `#Semver`.

```cue
import "github.com/cue-exp/cue_user_funcs/sprig"

snake: sprig.#Snakecase("HelloWorld")
```

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

## Future work

- **go.sum-based dependency locking.** Currently, the generated temporary Go
  module downloads dependencies based solely on the module@version specified in
  inject names. In the future, the consuming CUE module could maintain a
  `go.sum`-style lock file that pins the expected cryptographic hashes of
  downloaded Go modules. This would provide stronger guarantees that the Go code
  being injected has not been tampered with or substituted, bringing the safety
  model closer to what Go itself provides for its own dependencies.
