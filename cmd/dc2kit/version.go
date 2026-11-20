package main

import "runtime/debug"

// version is set by release builds with -ldflags "-X main.version=...".
var version = ""

// buildVersion reports the release version, or the module version Go
// recorded for `go install ...@vX`, or "devel" for a local build.
func buildVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "devel"
}
