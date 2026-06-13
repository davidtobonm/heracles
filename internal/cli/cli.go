package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/davidtobonm/heracles/internal/agent"
	"github.com/davidtobonm/heracles/internal/buildinfo"
	"github.com/davidtobonm/heracles/internal/control"
	"github.com/davidtobonm/heracles/internal/doctor"
	"github.com/davidtobonm/heracles/internal/mcp"
	"github.com/davidtobonm/heracles/internal/project"
)

const help = `Heracles coordinates agent-driven software delivery.

Usage:
  heracles [command]

Available Commands:
  heracles plan       Run or resume a Planning Stage
  heracles issues     Run or resume an Issue Stage
  heracles run        Run the defined Implementation Stage backlog
  heracles labor      Start or resume an end-to-end Labor
  heracles list       List durable Labors, issues, Change Sets, gates, logs, or evidence
  heracles inspect    Inspect one durable workflow record
  heracles mcp serve  Start the stdio MCP Control Surface
  heracles approve    Approve a Planning or Issue publication gate
  heracles reject     Reject a Planning or Issue publication gate for revision
  heracles retry      Retry a failed or blocked issue attempt
  heracles resume     Resume an interrupted or blocked Labor
  heracles cancel     Cancel a Labor
  heracles doctor     Validate a project before starting a Labor
  heracles init       Initialize a portable Project Configuration
  heracles version    Print Heracles version information
`

// Options supplies process-level dependencies to the CLI.
type Options struct {
	WorkingDirectory string
	DoctorSystem     doctor.System
	Control          control.Surface
	Input            io.Reader
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

	if args[0] == "doctor" {
		return runDoctor(args[1:], stdout, stderr, options)
	}

	if args[0] == "version" {
		fmt.Fprintln(stdout, buildinfo.String())
		return 0
	}

	if args[0] == "mcp" {
		return runMCP(args[1:], stdout, stderr, options)
	}

	for _, command := range []string{"plan", "issues", "run", "labor", "list", "inspect", "approve", "reject", "retry", "resume", "cancel"} {
		if args[0] == command {
			return runControl(command, args[1:], stdout, stderr, options)
		}
	}

	fmt.Fprintf(stderr, "unknown command %q\n", args[0])
	return 2
}

func runMCP(args []string, stdout, stderr io.Writer, options Options) int {
	if len(args) == 0 || args[0] != "serve" {
		fmt.Fprintln(stderr, "usage: heracles mcp serve [--config path]")
		return 2
	}
	flags := flag.NewFlagSet("heracles mcp serve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "select Project Configuration path")
	if err := flags.Parse(args[1:]); errors.Is(err, flag.ErrHelp) {
		return 0
	} else if err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "heracles mcp serve does not accept positional arguments")
		return 2
	}
	surface := options.Control
	owned := false
	if surface == nil {
		surface = control.NewDynamic(options.WorkingDirectory, *configPath)
		owned = true
	}
	if owned {
		defer surface.Close()
	}
	input := options.Input
	if input == nil {
		input = os.Stdin
	}
	if err := (mcp.Server{Surface: surface, Version: buildinfo.String()}).Serve(context.Background(), input, stdout); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func runControl(command string, args []string, stdout, stderr io.Writer, options Options) int {
	flags := flag.NewFlagSet("heracles "+command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "select Project Configuration path")
	jsonOutput := flags.Bool("json", false, "emit stable machine-readable JSON")
	id := flags.String("id", "", "workflow record ID")
	problem := flags.String("problem", "", "problem description")
	prdPath := flags.String("prd", "", "approved PRD path")
	reason := flags.String("reason", "", "decision or operation reason")
	if err := flags.Parse(interspersedFlags(args)); errors.Is(err, flag.ErrHelp) {
		return 0
	} else if err != nil {
		return 2
	}

	operation := control.Operation{Name: command, ID: *id, Problem: *problem, Reason: *reason}
	positionals := flags.Args()
	switch command {
	case "list":
		if len(positionals) != 1 {
			fmt.Fprintln(stderr, "heracles list requires one kind")
			return 2
		}
		operation.Kind = positionals[0]
	case "inspect":
		if len(positionals) != 2 {
			fmt.Fprintln(stderr, "heracles inspect requires kind and ID")
			return 2
		}
		operation.Kind, operation.ID = positionals[0], positionals[1]
	case "approve", "reject":
		if len(positionals) != 2 {
			fmt.Fprintf(stderr, "heracles %s requires gate kind and ID\n", command)
			return 2
		}
		operation.Kind, operation.ID, operation.Decision = positionals[0], positionals[1], command
	case "retry", "resume", "cancel":
		if len(positionals) != 1 {
			fmt.Fprintf(stderr, "heracles %s requires one ID\n", command)
			return 2
		}
		operation.ID = positionals[0]
	default:
		if len(positionals) != 0 {
			fmt.Fprintf(stderr, "heracles %s does not accept positional arguments\n", command)
			return 2
		}
	}
	if command == "issues" {
		if *prdPath != "" {
			contents, err := os.ReadFile(*prdPath)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			operation.PRD = string(contents)
		}
	}
	if command == "labor" && (operation.ID == "" || operation.Problem == "") {
		fmt.Fprintln(stderr, "heracles labor requires --id and --problem")
		return 2
	}
	if (command == "plan" || command == "issues") && operation.ID == "" {
		fmt.Fprintf(stderr, "heracles %s requires --id\n", command)
		return 2
	}

	surface, owned, err := controlSurface(options, *configPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if owned {
		defer surface.Close()
	}
	result, err := surface.Execute(context.Background(), operation)
	if err != nil {
		if *jsonOutput {
			_ = json.NewEncoder(stdout).Encode(control.Result{Operation: operation.Name, Kind: operation.Kind, ID: operation.ID, Status: "error", Data: map[string]string{"error": err.Error()}})
		}
		fmt.Fprintln(stderr, err)
		return 1
	}
	if *jsonOutput {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	fmt.Fprintf(stdout, "%s", result.Operation)
	if result.ID != "" {
		fmt.Fprintf(stdout, " %s", result.ID)
	}
	fmt.Fprintf(stdout, ": %s\n", result.Status)
	if result.Data != nil && (command == "list" || command == "inspect") {
		contents, _ := json.MarshalIndent(result.Data, "", "  ")
		fmt.Fprintf(stdout, "%s\n", contents)
	}
	return 0
}

func interspersedFlags(args []string) []string {
	valueFlags := map[string]bool{"--config": true, "--id": true, "--problem": true, "--prd": true, "--reason": true}
	var flags []string
	var positionals []string
	for index := 0; index < len(args); index++ {
		argument := args[index]
		if argument == "--json" || argument == "-json" || argument == "--help" || argument == "-h" {
			flags = append(flags, argument)
			continue
		}
		if valueFlags[argument] || valueFlags["--"+argument] {
			flags = append(flags, argument)
			if index+1 < len(args) {
				index++
				flags = append(flags, args[index])
			}
			continue
		}
		if len(argument) > 2 && argument[:2] == "--" {
			flags = append(flags, argument)
			continue
		}
		positionals = append(positionals, argument)
	}
	return append(flags, positionals...)
}

func controlSurface(options Options, explicitConfig string) (control.Surface, bool, error) {
	if options.Control != nil {
		return options.Control, false, nil
	}
	path, err := project.Discover(options.WorkingDirectory, explicitConfig)
	if err != nil {
		return nil, false, err
	}
	loaded, err := project.Load(path)
	if err != nil {
		return nil, false, err
	}
	surface, err := control.NewLocal(context.Background(), loaded)
	return surface, true, err
}

func runDoctor(args []string, stdout, stderr io.Writer, options Options) int {
	flags := flag.NewFlagSet("heracles doctor", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "select Project Configuration path")
	if err := flags.Parse(args); errors.Is(err, flag.ErrHelp) {
		return 0
	} else if err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "heracles doctor does not accept positional arguments")
		return 2
	}

	path, err := project.Discover(options.WorkingDirectory, *configPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	loaded, err := project.Load(path)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	system := options.DoctorSystem
	if system == nil {
		system = doctor.OSSystem{}
	}
	report := doctor.Check(context.Background(), loaded, agent.DefaultRegistry(), system)
	fmt.Fprint(stdout, report.String())
	if !report.OK {
		return 1
	}
	return 0
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
