package main

import (
	"embed"
	"encoding/json"
	"fmt"
	goast "go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"text/template"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/load"
)

//go:embed _template/main.go
var templateFS embed.FS

func main() {
	os.Exit(main1())
}

func main1() int {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	return 0
}

func run() error {
	if len(os.Args) < 3 || os.Args[1] != "export" {
		return fmt.Errorf("usage: %s export <directory>", os.Args[0])
	}
	dir := os.Args[2]

	// Make dir absolute so the generated binary can find it.
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}

	// Load the CUE package to discover @inject attributes.
	cfg := &load.Config{Dir: absDir}
	instances := load.Instances([]string{"."}, cfg)
	if len(instances) == 0 {
		return fmt.Errorf("no instances found in %s", dir)
	}
	inst := instances[0]
	if inst.Err != nil {
		return inst.Err
	}

	// Walk all instances (including transitive deps) to find @inject names.
	injectNames := collectInjectNames(inst)
	if len(injectNames) == 0 {
		return fmt.Errorf("no @inject attributes found")
	}

	// Parse inject names into module requirements and function references.
	funcs, err := parseInjectNames(injectNames)
	if err != nil {
		return err
	}

	// Create a temporary directory for the generated module.
	tmpDir, err := os.MkdirTemp("", "cue-user-funcs-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	// Write the template main.go.
	tmplMain, err := templateFS.ReadFile("_template/main.go")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), tmplMain, 0o666); err != nil {
		return err
	}

	// Write go.mod.
	if err := writeGoMod(tmpDir, funcs); err != nil {
		return err
	}

	// Write a stub file with blank imports so go mod tidy downloads
	// the function packages.
	if err := writeStubFile(tmpDir, funcs); err != nil {
		return err
	}

	// Run go mod tidy to download modules.
	if err := goCmd(tmpDir, "mod", "tidy"); err != nil {
		return fmt.Errorf("go mod tidy: %w", err)
	}

	// Resolve function signatures from the downloaded Go source.
	// TODO: cache the generated program on a content-addressed hash of inputs.
	sigs, err := resolveFuncSigs(tmpDir, funcs)
	if err != nil {
		return err
	}

	// Replace stub with the generated register.go containing typed wrappers.
	if err := os.Remove(filepath.Join(tmpDir, "stub.go")); err != nil {
		return err
	}
	if err := writeRegisterFile(tmpDir, funcs, sigs); err != nil {
		return err
	}

	// Build the generated module.
	binPath := filepath.Join(tmpDir, "cue-user-funcs")
	if err := goCmd(tmpDir, "build", "-o", binPath, "."); err != nil {
		return fmt.Errorf("go build: %w", err)
	}

	// Exec the built binary, replacing the current process.
	args := []string{binPath, "export", absDir}
	return syscall.Exec(binPath, args, os.Environ())
}

// collectInjectNames walks the instance and its transitive imports,
// returning all @inject name values from files that have @extern(inject).
func collectInjectNames(root *build.Instance) []string {
	var names []string
	seen := map[string]bool{}
	var walk func(inst *build.Instance)
	walk = func(inst *build.Instance) {
		if seen[inst.ImportPath] {
			return
		}
		seen[inst.ImportPath] = true

		for _, f := range inst.Files {
			names = append(names, extractInjectNames(f)...)
		}
		for _, imp := range inst.Imports {
			walk(imp)
		}
	}
	walk(root)
	return names
}

// extractInjectNames returns all @inject name values from a file
// that has a file-level @extern(inject) attribute.
func extractInjectNames(f *ast.File) []string {
	// Check for file-level @extern(inject).
	hasExtern := false
	for _, d := range f.Decls {
		attr, ok := d.(*ast.Attribute)
		if !ok {
			continue
		}
		key, body := attr.Split()
		if key == "extern" && body == "inject" {
			hasExtern = true
			break
		}
	}
	if !hasExtern {
		return nil
	}

	// Collect @inject names from fields.
	var names []string
	for _, d := range f.Decls {
		field, ok := d.(*ast.Field)
		if !ok {
			continue
		}
		for _, a := range field.Attrs {
			key, body := a.Split()
			if key != "inject" {
				continue
			}
			name := parseInjectAttrName(body)
			if name != "" {
				names = append(names, name)
			}
		}
	}
	return names
}

// parseInjectAttrName extracts the name value from an @inject attribute body
// like `name="golang.org/x/mod@v0.33.0/semver.IsValid"`.
func parseInjectAttrName(body string) string {
	// Body is: name="value"
	prefix := `name="`
	if !strings.HasPrefix(body, prefix) {
		return ""
	}
	rest := body[len(prefix):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return ""
	}
	return rest[:end]
}

// funcRef represents a parsed inject name.
type funcRef struct {
	// InjectName is the full inject name, e.g. "golang.org/x/mod@v0.33.0/semver.IsValid".
	InjectName string
	// Module is the Go module path, e.g. "golang.org/x/mod".
	Module string
	// Version is the module version, e.g. "v0.33.0".
	Version string
	// ImportPath is the full Go import path, e.g. "golang.org/x/mod/semver".
	ImportPath string
	// FuncName is the function name, e.g. "IsValid".
	FuncName string
}

// injectNameRe matches "module@version/subpath.Func" or "module@version.Func" (no subpath).
var injectNameRe = regexp.MustCompile(`^(.+)@(v[^/]+?)(?:/(.+))?\.([A-Z]\w*)$`)

func parseInjectNames(names []string) ([]funcRef, error) {
	var funcs []funcRef
	for _, name := range names {
		m := injectNameRe.FindStringSubmatch(name)
		if m == nil {
			return nil, fmt.Errorf("cannot parse inject name %q", name)
		}
		module := m[1]
		version := m[2]
		subpath := m[3]
		funcName := m[4]

		importPath := module
		if subpath != "" {
			importPath = module + "/" + subpath
		}

		funcs = append(funcs, funcRef{
			InjectName: name,
			Module:     module,
			Version:    version,
			ImportPath: importPath,
			FuncName:   funcName,
		})
	}
	return funcs, nil
}

func writeGoMod(tmpDir string, funcs []funcRef) error {
	goMod := `module _cue_user_funcs_generated

go 1.25.0

require cuelang.org/go v0.16.0

replace cuelang.org/go v0.16.0 => github.com/cue-exp/cue v0.0.0-20260306105357-d03fc6701a45
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0o666); err != nil {
		return err
	}

	// Add a require for each unique module@version.
	seen := map[string]bool{}
	for _, f := range funcs {
		key := f.Module + "@" + f.Version
		if seen[key] {
			continue
		}
		seen[key] = true
		if err := goCmd(tmpDir, "mod", "edit", "-require="+key); err != nil {
			return fmt.Errorf("go mod edit -require=%s: %w", key, err)
		}
	}
	return nil
}

// writeStubFile writes a temporary Go file with blank imports for each
// function package, so that go mod tidy downloads the required modules.
func writeStubFile(tmpDir string, funcs []funcRef) error {
	var buf strings.Builder
	buf.WriteString("package main\n\nimport (\n")
	seen := map[string]bool{}
	for _, f := range funcs {
		if seen[f.ImportPath] {
			continue
		}
		seen[f.ImportPath] = true
		fmt.Fprintf(&buf, "\t_ %q\n", f.ImportPath)
	}
	buf.WriteString(")\n")
	return os.WriteFile(filepath.Join(tmpDir, "stub.go"), []byte(buf.String()), 0o666)
}

// funcSig holds the parsed signature of a Go function.
type funcSig struct {
	Params       []string // Go type names, e.g. ["string", "int"]
	ReturnsError bool     // true if the last return value is error
}

type goListResult struct {
	Dir string
}

// goListDir runs "go list -json" to find the source directory for a package.
func goListDir(tmpDir, importPath string) (string, error) {
	cmd := exec.Command("go", "list", "-json", importPath)
	cmd.Dir = tmpDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("go list -json %s: %w", importPath, err)
	}
	var result goListResult
	if err := json.Unmarshal(out, &result); err != nil {
		return "", err
	}
	return result.Dir, nil
}

// resolveFuncSigs resolves the Go function signatures for all inject functions
// by running "go list -json" to locate package source and parsing the Go AST.
func resolveFuncSigs(tmpDir string, funcs []funcRef) (map[string]*funcSig, error) {
	// Resolve package directories, one go list call per unique import path.
	pkgDirs := map[string]string{}
	for _, f := range funcs {
		if _, ok := pkgDirs[f.ImportPath]; ok {
			continue
		}
		dir, err := goListDir(tmpDir, f.ImportPath)
		if err != nil {
			return nil, err
		}
		pkgDirs[f.ImportPath] = dir
	}

	// Parse each function's signature from the Go source.
	sigs := map[string]*funcSig{}
	for _, f := range funcs {
		sig, err := parseFuncSig(pkgDirs[f.ImportPath], f.FuncName)
		if err != nil {
			return nil, fmt.Errorf("resolving %s: %w", f.InjectName, err)
		}
		sigs[f.InjectName] = sig
	}
	return sigs, nil
}

// parseFuncSig parses Go source files in pkgDir to extract the signature
// of the named function.
func parseFuncSig(pkgDir, funcName string) (*funcSig, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, pkgDir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		return nil, err
	}
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*goast.FuncDecl)
				if !ok || fn.Name.Name != funcName || fn.Recv != nil {
					continue
				}
				sig := &funcSig{}
				for _, param := range fn.Type.Params.List {
					typeName, err := typeExprName(param.Type)
					if err != nil {
						return nil, fmt.Errorf("function %s: %w", funcName, err)
					}
					// Handle grouped params like (a, b string).
					n := len(param.Names)
					if n == 0 {
						n = 1
					}
					for range n {
						sig.Params = append(sig.Params, typeName)
					}
				}
				if fn.Type.Results != nil {
					results := fn.Type.Results.List
					last := results[len(results)-1]
					if ident, ok := last.Type.(*goast.Ident); ok && ident.Name == "error" {
						sig.ReturnsError = true
					}
				}
				return sig, nil
			}
		}
	}
	return nil, fmt.Errorf("function %s not found in %s", funcName, pkgDir)
}

// typeExprName extracts the Go type name from an AST type expression.
// Only built-in types (string, int, int64, bool, float64) are supported.
func typeExprName(expr goast.Expr) (string, error) {
	ident, ok := expr.(*goast.Ident)
	if !ok {
		return "", fmt.Errorf("unsupported type expression %T (only built-in types are supported)", expr)
	}
	return ident.Name, nil
}

// convExpr returns a Go expression that converts a PureFunc any-typed
// parameter to the concrete Go type the function expects.
func convExpr(paramName, goType string) (string, error) {
	switch goType {
	case "string":
		return paramName + ".(string)", nil
	case "bool":
		return paramName + ".(bool)", nil
	case "int":
		return "int(" + paramName + ".(int64))", nil
	case "int64":
		return paramName + ".(int64)", nil
	case "float64":
		return paramName + ".(float64)", nil
	default:
		return "", fmt.Errorf("unsupported parameter type %q", goType)
	}
}

var registerTmpl = template.Must(template.New("register").Parse(`package main

import (
	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
{{- range .Imports}}
	{{.Alias}} "{{.Path}}"
{{- end}}
)

func registerAll(j *cuecontext.Injector) {
{{- range .Funcs}}
	j.Register("{{.InjectName}}", cue.PureFunc{{.Arity}}(func({{.ParamDecl}}) (any, error) {
{{- if .ReturnsError}}
		return {{.CallExpr}}
{{- else}}
		return {{.CallExpr}}, nil
{{- end}}
	}, cue.Name("{{.InjectName}}")))
{{- end}}
}
`))

type registerData struct {
	Imports []importEntry
	Funcs   []registerFuncEntry
}

type importEntry struct {
	Alias string
	Path  string
}

type registerFuncEntry struct {
	InjectName   string
	Arity        int
	ParamDecl    string
	CallExpr     string
	ReturnsError bool
}

func writeRegisterFile(tmpDir string, funcs []funcRef, sigs map[string]*funcSig) error {
	// Assign unique import aliases.
	importAliases := map[string]string{} // importPath -> alias
	aliasCounter := 0
	for _, f := range funcs {
		if _, ok := importAliases[f.ImportPath]; ok {
			continue
		}
		aliasCounter++
		importAliases[f.ImportPath] = fmt.Sprintf("pkg%d", aliasCounter)
	}

	var imports []importEntry
	seen := map[string]bool{}
	for _, f := range funcs {
		if seen[f.ImportPath] {
			continue
		}
		seen[f.ImportPath] = true
		imports = append(imports, importEntry{
			Alias: importAliases[f.ImportPath],
			Path:  f.ImportPath,
		})
	}

	var entries []registerFuncEntry
	for _, f := range funcs {
		sig := sigs[f.InjectName]
		alias := importAliases[f.ImportPath]

		// Build parameter declaration, e.g. "a0, a1 any".
		paramNames := make([]string, len(sig.Params))
		for i := range sig.Params {
			paramNames[i] = fmt.Sprintf("a%d", i)
		}
		paramDecl := strings.Join(paramNames, ", ") + " any"

		// Build call expression with typed conversions,
		// e.g. "pkg1.IsValid(a0.(string))".
		argExprs := make([]string, len(sig.Params))
		for i, paramType := range sig.Params {
			expr, err := convExpr(fmt.Sprintf("a%d", i), paramType)
			if err != nil {
				return fmt.Errorf("function %s param %d: %w", f.InjectName, i, err)
			}
			argExprs[i] = expr
		}
		callExpr := fmt.Sprintf("%s.%s(%s)", alias, f.FuncName, strings.Join(argExprs, ", "))

		entries = append(entries, registerFuncEntry{
			InjectName:   f.InjectName,
			Arity:        len(sig.Params),
			ParamDecl:    paramDecl,
			CallExpr:     callExpr,
			ReturnsError: sig.ReturnsError,
		})
	}

	out, err := os.Create(filepath.Join(tmpDir, "register.go"))
	if err != nil {
		return err
	}
	defer out.Close()
	return registerTmpl.Execute(out, registerData{
		Imports: imports,
		Funcs:   entries,
	})
}

func goCmd(dir string, args ...string) error {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
