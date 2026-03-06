# cue-user-funcs

A Go program that emulates `cue export`, extended with user-provided functions
exposing [`golang.org/x/mod/semver`](https://pkg.go.dev/golang.org/x/mod/semver)
to CUE.

This uses CUE's WIP user-provided functions and value injection proposals
([#4293](https://github.com/cue-lang/proposal/blob/main/designs/4293-user-functions-and-validators.md),
[#4294](https://github.com/cue-lang/proposal/blob/main/designs/4294-value-injection.md))
via a [fork](https://github.com/myitcvforks/cue/tree/user_funcs_etc) of
`cuelang.org/go`.

## Usage

```
go run . export <directory>
```

The directory must contain a CUE package. The program loads it, evaluates it
(injecting semver functions), and prints the result as JSON.

## Available functions

The following `semver` functions are injected and available via `@inject` attributes:

| Injection name       | Signature                        | Description                          |
|----------------------|----------------------------------|--------------------------------------|
| `semver.IsValid`     | `(v: string) -> bool`            | Reports whether v is valid semver    |
| `semver.Compare`     | `(v: string, w: string) -> int`  | -1, 0, or +1 comparison             |
| `semver.Canonical`   | `(v: string) -> string`          | Canonical formatting                 |
| `semver.Major`       | `(v: string) -> string`          | Major version prefix (e.g. "v1")    |
| `semver.MajorMinor`  | `(v: string) -> string`          | Major.minor prefix (e.g. "v1.2")   |
| `semver.Prerelease`  | `(v: string) -> string`          | Prerelease suffix (e.g. "-beta")   |
| `semver.Build`       | `(v: string) -> string`          | Build suffix (e.g. "+meta")        |

## Example

See [`testdata/example.cue`](testdata/example.cue) for a working example:

```
go run . export ./testdata
```

Output:

```json
{
    "build": "+build456",
    "canonical": "v1.2.3-beta",
    "compare": -1,
    "isValid": true,
    "major": "v1",
    "majorMinor": "v1.2",
    "prerelease": "-beta",
    "version": "v1.2.3-beta+build456",
    "versions": {
        "a": "v2.0.0",
        "aIsNewer": true,
        "b": "v1.9.0"
    }
}
```

## CUE package setup

CUE files must use `@extern(inject)` at the file level and `@inject(name=...)`
on fields to receive the injected functions:

```cue
@extern(inject)

package mypackage

#semverIsValid: _ @inject(name="semver.IsValid")

result: #semverIsValid("v1.0.0")
```
