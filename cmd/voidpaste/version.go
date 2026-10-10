package main

import "fmt"
import "runtime/debug"

// Set by the release workflow with -ldflags. Defaults keep go install and
// local source builds useful without pretending they are tagged releases.
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func effectiveVersion() string {
	v := version
	if v == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			v = info.Main.Version
		}
	}
	return v
}

func versionOutput() string {
	return fmt.Sprintf("voidpaste %s\ncommit: %s\nbuilt: %s", effectiveVersion(), commit, buildDate)
}
