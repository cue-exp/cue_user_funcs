@extern(inject)

package example

#semverIsValid:    _ @inject(name="semver.IsValid")
#semverCompare:    _ @inject(name="semver.Compare")
#semverMajor:      _ @inject(name="semver.Major")
#semverMajorMinor: _ @inject(name="semver.MajorMinor")
#semverCanonical:  _ @inject(name="semver.Canonical")
#semverPrerelease: _ @inject(name="semver.Prerelease")
#semverBuild:      _ @inject(name="semver.Build")

version: "v1.2.3-beta+build456"

isValid:    #semverIsValid(version)
canonical:  #semverCanonical(version)
major:      #semverMajor(version)
majorMinor: #semverMajorMinor(version)
prerelease: #semverPrerelease(version)
build:      #semverBuild(version)

compare: #semverCompare("v1.2.3", "v1.3.0")

versions: {
	a: "v2.0.0"
	b: "v1.9.0"
	aIsNewer: #semverCompare(a, b) > 0
}
