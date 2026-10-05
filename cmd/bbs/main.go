// Command bbs is the babysit CLI. It is a multicall binary: when invoked
// through a `bbs-<name>` compat symlink it dispatches to the `<name>`
// subcommand, so the legacy `bbs-config` name runs `bbs config`.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/reallongnguyen/babysit/internal/cmd"
)

func main() {
	root := cmd.NewRootCmd()
	if base := filepath.Base(os.Args[0]); strings.HasPrefix(base, "bbs-") {
		// Windows executables carry .exe: bbs-config.exe → "config".
		sub := strings.TrimSuffix(strings.TrimPrefix(base, "bbs-"), ".exe")
		root.SetArgs(append([]string{sub}, os.Args[1:]...))
	}
	if err := root.Execute(); err != nil {
		if msg := err.Error(); msg != "" {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			if usageError(msg) && !strings.Contains(strings.ToLower(msg), "help") {
				fmt.Fprintln(os.Stderr, "Run 'bbs --help' for available commands.")
			}
		}
		os.Exit(1)
	}
}

func usageError(msg string) bool {
	s := strings.ToLower(msg)
	return strings.Contains(s, "unknown command") ||
		strings.Contains(s, "unknown flag") ||
		strings.Contains(s, "unknown shorthand") ||
		strings.Contains(s, "unknown setup option") ||
		strings.Contains(s, "unknown update arguments") ||
		strings.Contains(s, "invalid argument") ||
		strings.Contains(s, "accepts at most") ||
		strings.Contains(s, "usage:")
}
