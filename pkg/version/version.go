package version

import (
	"fmt"

	"runtime/debug"
)

// version provided by ldflags at compile-time
var version string

func GetVersion() string {
	if version == "" {
		if i, ok := debug.ReadBuildInfo(); ok {
			version = i.Main.Version
		}
	}
	return version
}

func PrintVersion() {
	fmt.Println(GetVersion())
}
