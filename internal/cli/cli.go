package cli

import (
	"fmt"
	"io"

	"github.com/davidtobonm/heracles/internal/buildinfo"
)

const help = `Heracles coordinates agent-driven software delivery.

Usage:
  heracles [command]

Available Commands:
  heracles version    Print Heracles version information
`

// Run executes the Heracles CLI and returns a process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Fprint(stdout, help)
		return 0
	}

	if args[0] == "version" {
		fmt.Fprintln(stdout, buildinfo.String())
		return 0
	}

	fmt.Fprintf(stderr, "unknown command %q\n", args[0])
	return 2
}
