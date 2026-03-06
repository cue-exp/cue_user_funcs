package main

import (
	"encoding/json"
	"fmt"
	"os"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
	"example.com/cue-user-funcs/sprig"
	"golang.org/x/mod/semver"
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
	ctx := cuecontext.New(cuecontext.Inject(j))

	// Register semver functions as user-provided functions.
	j.Register("golang.org/x/mod/semver.IsValid", cue.PureFunc1(func(v string) (bool, error) {
		return semver.IsValid(v), nil
	}, cue.Name("golang.org/x/mod/semver.IsValid")))

	j.Register("golang.org/x/mod/semver.Compare", cue.PureFunc2(func(v, w string) (int, error) {
		return semver.Compare(v, w), nil
	}, cue.Name("golang.org/x/mod/semver.Compare")))

	j.Register("golang.org/x/mod/semver.Major", cue.PureFunc1(func(v string) (string, error) {
		return semver.Major(v), nil
	}, cue.Name("golang.org/x/mod/semver.Major")))

	j.Register("golang.org/x/mod/semver.MajorMinor", cue.PureFunc1(func(v string) (string, error) {
		return semver.MajorMinor(v), nil
	}, cue.Name("golang.org/x/mod/semver.MajorMinor")))

	j.Register("golang.org/x/mod/semver.Canonical", cue.PureFunc1(func(v string) (string, error) {
		return semver.Canonical(v), nil
	}, cue.Name("golang.org/x/mod/semver.Canonical")))

	j.Register("golang.org/x/mod/semver.Prerelease", cue.PureFunc1(func(v string) (string, error) {
		return semver.Prerelease(v), nil
	}, cue.Name("golang.org/x/mod/semver.Prerelease")))

	j.Register("golang.org/x/mod/semver.Build", cue.PureFunc1(func(v string) (string, error) {
		return semver.Build(v), nil
	}, cue.Name("golang.org/x/mod/semver.Build")))

	// Register sprig string functions.
	j.Register("sprig.Title", cue.PureFunc1(func(s string) (string, error) {
		return sprig.Title(s), nil
	}, cue.Name("sprig.Title")))

	j.Register("sprig.Untitle", cue.PureFunc1(func(s string) (string, error) {
		return sprig.Untitle(s), nil
	}, cue.Name("sprig.Untitle")))

	j.Register("sprig.Substr", cue.PureFunc3(func(start, end int, s string) (string, error) {
		return sprig.Substr(start, end, s), nil
	}, cue.Name("sprig.Substr")))

	j.Register("sprig.Nospace", cue.PureFunc1(func(s string) (string, error) {
		return sprig.Nospace(s), nil
	}, cue.Name("sprig.Nospace")))

	j.Register("sprig.Trunc", cue.PureFunc2(func(n int, s string) (string, error) {
		return sprig.Trunc(n, s), nil
	}, cue.Name("sprig.Trunc")))

	j.Register("sprig.Abbrev", cue.PureFunc2(func(width int, s string) (string, error) {
		return sprig.Abbrev(width, s), nil
	}, cue.Name("sprig.Abbrev")))

	j.Register("sprig.Abbrevboth", cue.PureFunc3(func(left, right int, s string) (string, error) {
		return sprig.Abbrevboth(left, right, s), nil
	}, cue.Name("sprig.Abbrevboth")))

	j.Register("sprig.Initials", cue.PureFunc1(func(s string) (string, error) {
		return sprig.Initials(s), nil
	}, cue.Name("sprig.Initials")))

	j.Register("sprig.Wrap", cue.PureFunc2(func(width int, s string) (string, error) {
		return sprig.Wrap(width, s), nil
	}, cue.Name("sprig.Wrap")))

	j.Register("sprig.WrapWith", cue.PureFunc3(func(width int, sep, s string) (string, error) {
		return sprig.WrapWith(width, sep, s), nil
	}, cue.Name("sprig.WrapWith")))

	j.Register("sprig.Indent", cue.PureFunc2(func(spaces int, s string) (string, error) {
		return sprig.Indent(spaces, s), nil
	}, cue.Name("sprig.Indent")))

	j.Register("sprig.Nindent", cue.PureFunc2(func(spaces int, s string) (string, error) {
		return sprig.Nindent(spaces, s), nil
	}, cue.Name("sprig.Nindent")))

	j.Register("sprig.Snakecase", cue.PureFunc1(func(s string) (string, error) {
		return sprig.Snakecase(s), nil
	}, cue.Name("sprig.Snakecase")))

	j.Register("sprig.Camelcase", cue.PureFunc1(func(s string) (string, error) {
		return sprig.Camelcase(s), nil
	}, cue.Name("sprig.Camelcase")))

	j.Register("sprig.Kebabcase", cue.PureFunc1(func(s string) (string, error) {
		return sprig.Kebabcase(s), nil
	}, cue.Name("sprig.Kebabcase")))

	j.Register("sprig.Swapcase", cue.PureFunc1(func(s string) (string, error) {
		return sprig.Swapcase(s), nil
	}, cue.Name("sprig.Swapcase")))

	j.Register("sprig.Plural", cue.PureFunc3(func(one, many string, count int) (string, error) {
		return sprig.Plural(one, many, count), nil
	}, cue.Name("sprig.Plural")))

	// Register sprig semver functions.
	j.Register("sprig.SemverCompare", cue.PureFunc2(func(constraint, version string) (bool, error) {
		return sprig.SemverCompare(constraint, version)
	}, cue.Name("sprig.SemverCompare")))

	j.Register("sprig.Semver", cue.PureFunc1(func(version string) (*sprig.SemverVersion, error) {
		return sprig.Semver(version)
	}, cue.Name("sprig.Semver")))

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
