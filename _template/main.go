package main

import (
	"encoding/json"
	"fmt"
	"os"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
)

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
	registerAll(j)
	ctx := cuecontext.New(cuecontext.Inject(j))

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
