package main

import (
	"runtime/debug"
	"strings"
)

// version is set by the release build: -ldflags "-X main.version=2.3.0".
var version = ""

// programVersion is the release version; for a build from a checkout it is
// "dev" with the commit, so a bug report still says what was running.
func programVersion() string {
	if version != "" {
		return strings.TrimPrefix(version, "v")
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		rev, dirty := "", false
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				rev = s.Value
			case "vcs.modified":
				dirty = s.Value == "true"
			}
		}
		if len(rev) >= 7 {
			v := "dev-" + rev[:7]
			if dirty {
				v += "+"
			}
			return v
		}
	}
	return "dev"
}
