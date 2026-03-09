package main

import (
	"encoding/json"
	"flag"
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
	if len(os.Args) < 2 || os.Args[1] != "export" {
		return fmt.Errorf("usage: %s export [--test] <directory>", os.Args[0])
	}
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	testFlag := fs.Bool("test", false, "include @if(test) guarded CUE files")
	if err := fs.Parse(os.Args[2:]); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("usage: %s export [--test] <directory>", os.Args[0])
	}
	dir := fs.Arg(0)

	j := cuecontext.NewInjector()
	j.AllowAll()
	registerAll(j)
	ctx := cuecontext.New(cuecontext.Inject(j))

	cfg := &load.Config{Dir: dir}
	if *testFlag {
		cfg.Tags = append(cfg.Tags, "test")
		cfg.Tests = true
	}
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
