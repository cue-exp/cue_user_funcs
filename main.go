package main

import (
	"encoding/json"
	"fmt"
	"os"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
	"golang.org/x/mod/semver"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 3 || os.Args[1] != "export" {
		return fmt.Errorf("usage: %s export <directory>", os.Args[0])
	}
	dir := os.Args[2]

	j := cuecontext.NewInjector()
	j.AllowAll()
	ctx := cuecontext.New(cuecontext.Inject(j))

	// Register semver functions as user-provided functions.
	j.Register("semver.IsValid", cue.PureFunc1(func(v string) (bool, error) {
		return semver.IsValid(v), nil
	}, cue.Name("semver.IsValid")))

	j.Register("semver.Compare", cue.PureFunc2(func(v, w string) (int, error) {
		return semver.Compare(v, w), nil
	}, cue.Name("semver.Compare")))

	j.Register("semver.Major", cue.PureFunc1(func(v string) (string, error) {
		return semver.Major(v), nil
	}, cue.Name("semver.Major")))

	j.Register("semver.MajorMinor", cue.PureFunc1(func(v string) (string, error) {
		return semver.MajorMinor(v), nil
	}, cue.Name("semver.MajorMinor")))

	j.Register("semver.Canonical", cue.PureFunc1(func(v string) (string, error) {
		return semver.Canonical(v), nil
	}, cue.Name("semver.Canonical")))

	j.Register("semver.Prerelease", cue.PureFunc1(func(v string) (string, error) {
		return semver.Prerelease(v), nil
	}, cue.Name("semver.Prerelease")))

	j.Register("semver.Build", cue.PureFunc1(func(v string) (string, error) {
		return semver.Build(v), nil
	}, cue.Name("semver.Build")))

	// Load the CUE package from the specified directory.
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
