package version

import (
	"fmt"
	"regexp"
	"strings"

	"runtime/debug"

	"github.com/Masterminds/semver/v3"

	"github.com/distribution/reference"
)

// version provided by ldflags at compile-time
var version string

const (
	DevelVersion = "0.0.0-devel"
)

var (
	ytsaurusVersionPrefix = regexp.MustCompile(`^[a-z]+-`)
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

func MustParse(version string) *Version {
	return semver.MustParse(version)
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

func ParseYtsaurusVersion(version string) (*Version, error) {
	version = ytsaurusVersionPrefix.ReplaceAllString(version, "")
	return ParseVersion(version)
}

func ExtractImageTag(image string) string {
	ref, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return ""
	}
	if tagged, ok := ref.(reference.Tagged); ok {
		return tagged.Tag()
	}
	return ""
}

// ParseImageVersion returns semantic version parsed from image tag or nil.
func ParseImageVersion(image string) *Version {
	tag := ExtractImageTag(image)
	if tag == "" {
		return nil
	}
	ver, err := ParseYtsaurusVersion(tag)
	if err != nil {
		return nil
	}
	return ver
}

// LatestSpytQueryTrackerSparkVersion selects and validates the latest Spark version.
func LatestSpytQueryTrackerSparkVersion(sparkVersions []string) (string, error) {
	var latest *Version
	for _, sparkVersion := range sparkVersions {
		if sparkVersion == "" {
			continue
		}
		parsed, err := ParseVersion(sparkVersion)
		if err != nil {
			return "", fmt.Errorf("invalid Spark version %q: %w", sparkVersion, err)
		}
		if latest == nil || parsed.GreaterThan(latest) {
			latest = parsed
		}
	}
	if latest == nil {
		return "", nil
	}
	sparkVersion := latest.Original()
	if strings.HasPrefix(sparkVersion, "3.5") || strings.HasPrefix(sparkVersion, "4.") {
		return sparkVersion, nil
	}
	return "", fmt.Errorf("incompatible Spark version %q: SPYT query tracker supports only Spark 3.5.x and 4.x", sparkVersion)
}
