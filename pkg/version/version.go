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
	return semver.StrictNewVersion(version)
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
	ver, err := ParseVersion(GetVersion())
	if err != nil {
		ver = semver.New(0, 0, 0, "malformed", "")
	}
	return ver
}
