package main

import (
	"errors"
	"os"

	"github.com/gronitab/git-release/internal/build"
	"github.com/gronitab/git-release/internal/commands"
)

// These vars are populated by GoReleaser via -ldflags.
// They live here so the linker path matches: main.version etc.
//
// The initializers MUST be string CONSTANTS. `-X main.version=...` patches the
// variable's initial data, but a non-constant initializer (e.g. `= build.Version`)
// makes the compiler emit a package init that runs AFTER the linker's patch and
// overwrites it — which is why every binary built before 2026-08-31 reported
// "dev". The release workflow now asserts the stamped binary prints its tag.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	build.Version = version
	build.Commit = commit
	build.Date = date

	err := commands.Root().Execute()
	if err == nil {
		os.Exit(0)
	}
	if errors.Is(err, commands.ErrAbort) {
		os.Exit(1)
	}
	os.Exit(2)
}
