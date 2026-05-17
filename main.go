package main

import (
	"os"

	"claude-skill-profiles/cmd"
)

func main() {
	// Match ait/ant: rewrite a top-level --version into the version
	// subcommand so `csp --version` and `csp version` produce identical
	// output (including the GitHub update check).
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		os.Args[1] = "version"
	}
	cmd.Execute()
}
