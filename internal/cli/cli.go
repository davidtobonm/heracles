package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/davidtobonm/heracles/internal/buildinfo"
	"github.com/davidtobonm/heracles/internal/project"
)

const help = `Heracles coordinates agent-driven software delivery.

Usage:
  heracles [command]

Available Commands:
  heracles init       Initialize a portable Project Configuration
  heracles version    Print Heracles version information
`

// Options supplies process-level dependencies to the CLI.
type Options struct {
	WorkingDirectory string
}

// Run executes the Heracles CLI and returns a process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	return RunWithOptions(args, stdout, stderr, Options{})
}

// RunWithOptions executes the Heracles CLI with explicit process-level options.
func RunWithOptions(args []string, stdout, stderr io.Writer, options Options) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Fprint(stdout, help)
		return 0
	}

	if args[0] == "init" {
		return runInit(args[1:], stdout, stderr, options)
	}

	if args[0] == "version" {
		fmt.Fprintln(stdout, buildinfo.String())
		return 0
	}

	fmt.Fprintf(stderr, "unknown command %q\n", args[0])
	return 2
}

func runInit(args []string, stdout, stderr io.Writer, options Options) int {
	flags := flag.NewFlagSet("heracles init", flag.ContinueOnError)
	flags.SetOutput(stderr)

	var repositories repeatedString
	configPath := flags.String("config", "", "write Project Configuration to path")
	tracker := flags.String("tracker", "", "GitHub Issue Tracker as owner/repository")
	flags.Var(&repositories, "repo", "Target Repository path; repeat for multiple repositories")

	if err := flags.Parse(args); errors.Is(err, flag.ErrHelp) {
		return 0
	} else if err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "heracles init does not accept positional arguments")
		return 2
	}

	result, err := project.Initialize(context.Background(), project.InitOptions{
		WorkingDirectory: options.WorkingDirectory,
		ConfigPath:       *configPath,
		Tracker:          *tracker,
		Repositories:     repositories,
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	fmt.Fprintf(stdout, "Initialized Project Configuration at %s\n", result.Path)
	return 0
}

type repeatedString []string

func (values *repeatedString) String() string {
	return fmt.Sprint([]string(*values))
}

func (values *repeatedString) Set(value string) error {
	*values = append(*values, value)
	return nil
}
