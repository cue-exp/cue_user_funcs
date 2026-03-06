package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
)

// funcsToRegister is populated by the generated register.go file.
var funcsToRegister map[string]any

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

	j := cuecontext.NewInjector()
	j.AllowAll()
	ctx := cuecontext.New(cuecontext.Inject(j))

	for name, fn := range funcsToRegister {
		if err := registerFunc(j, name, fn); err != nil {
			return fmt.Errorf("registering %s: %v", name, err)
		}
	}

	cfg := &load.Config{Dir: dir}
	instances := load.Instances([]string{"."}, cfg)
	if len(instances) == 0 {
		return fmt.Errorf("no instances found in %s", dir)
	}
	inst := instances[0]
	if inst.Err != nil {
		return inst.Err
	}

	v := ctx.BuildInstance(inst)
	if err := v.Err(); err != nil {
		return err
	}
	if err := v.Validate(cue.Concrete(true)); err != nil {
		return err
	}

	var out any
	if err := v.Decode(&out); err != nil {
		return err
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "    ")
	return enc.Encode(out)
}

var errorType = reflect.TypeOf((*error)(nil)).Elem()

func registerFunc(j *cuecontext.Injector, name string, fn any) error {
	fv := reflect.ValueOf(fn)
	ft := fv.Type()
	if ft.Kind() != reflect.Func {
		return fmt.Errorf("not a function: %T", fn)
	}

	numIn := ft.NumIn()
	numOut := ft.NumOut()
	returnsError := numOut == 2 && ft.Out(1).Implements(errorType)

	call := func(args []any) (any, error) {
		in := make([]reflect.Value, numIn)
		for i := range numIn {
			in[i] = reflect.ValueOf(args[i])
			if in[i].Type() != ft.In(i) {
				in[i] = in[i].Convert(ft.In(i))
			}
		}
		out := fv.Call(in)
		result := out[0].Interface()
		if returnsError && !out[1].IsNil() {
			return nil, out[1].Interface().(error)
		}
		return result, nil
	}

	switch numIn {
	case 1:
		j.Register(name, cue.PureFunc1(func(a0 any) (any, error) {
			return call([]any{a0})
		}, cue.Name(name)))
	case 2:
		j.Register(name, cue.PureFunc2(func(a0, a1 any) (any, error) {
			return call([]any{a0, a1})
		}, cue.Name(name)))
	case 3:
		j.Register(name, cue.PureFunc3(func(a0, a1, a2 any) (any, error) {
			return call([]any{a0, a1, a2})
		}, cue.Name(name)))
	case 4:
		j.Register(name, cue.PureFunc4(func(a0, a1, a2, a3 any) (any, error) {
			return call([]any{a0, a1, a2, a3})
		}, cue.Name(name)))
	default:
		return fmt.Errorf("unsupported function arity %d", numIn)
	}
	return nil
}
