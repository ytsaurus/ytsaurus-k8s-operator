package version

import (
	"fmt"

	"runtime/debug"

	"github.com/Masterminds/semver/v3"
)

// version provided by ldflags at compile-time
var version string

const (
	DevelVersion = "0.0.0-devel"
)

func GetVersion() string {
	if version == "" {
		if i, ok := debug.ReadBuildInfo(); ok {
			// "(devel)" for local run
			// "vX.Y.Z" for tagged run or go install
			// or "v(0.0.0|X.Y.Z)-[0.]YYYMMDDhhmmss-(digest)[+dirty]"
			// See: https://go.dev/ref/mod#versions
			version = i.Main.Version
			if version == "(devel)" {
				version = DevelVersion
			}
		}
	}
	return version
}

func PrintVersion() {
	fmt.Println(GetVersion())
}

type Version = semver.Version

func ParseVersion(version string) (*Version, error) {
	return semver.NewVersion(version)
}

type Constraints = semver.Constraints

func ParseConstraints(constraints string) (*Constraints, error) {
	cs, err := semver.NewConstraint(constraints)
	if err != nil {
		return nil, err
	}
	cs.IncludePrerelease = true
	return cs, nil
}

func GetParsedVersion() *Version {
	version := GetVersion()
	ver, err := ParseVersion(version)
	if err != nil {
		ver = semver.New(0, 0, 0, "malformed", version)
	}
	return ver
}
