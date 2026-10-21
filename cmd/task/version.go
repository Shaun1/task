package main

import (
	"runtime/debug"
	"strings"
)

// version is set at build time: -ldflags "-X main.version=$(git describe ...)".
// When empty (e.g. plain `go build` or `go install`), buildVersion falls back
// to the VCS info Go embeds in the binary.
var version string

func buildVersion() string {
	if version != "" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	return versionFromBuildInfo(info)
}

func versionFromBuildInfo(info *debug.BuildInfo) string {
	var rev string
	var dirty bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if rev == "" {
		return "dev"
	}
	if len(rev) > 7 {
		rev = rev[:7]
	}
	if dirty {
		rev += "-dirty"
	}
	return strings.TrimSpace("dev-" + rev)
}
