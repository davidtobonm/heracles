package mcp_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/davidtobonm/heracles/internal/cli"
	"github.com/davidtobonm/heracles/internal/control"
	"github.com/davidtobonm/heracles/internal/mcp"
)

func TestServerImplementsInitializeToolsAndSharedControlCalls(t *testing.T) {
	t.Parallel()

	surface := &fakeSurface{}
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"heracles_resume","arguments":{"id":"labor-1"}}}`,
	}, "\n") + "\n"
	var output bytes.Buffer
	if err := (mcp.Server{Surface: surface, Version: "test"}).Serve(context.Background(), strings.NewReader(input), &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("responses = %d, want no response for initialized notification: %s", len(lines), output.String())
	}
	for _, expected := range []string{`"protocolVersion":"2025-11-25"`, `"tools"`, `"heracles_labor"`, `"structuredContent"`, `"isError":false`} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("output does not contain %q: %s", expected, output.String())
		}
	}
	for _, forbidden := range []string{"shell", "exec", "command"} {
		if strings.Contains(output.String(), `"heracles_`+forbidden+`"`) {
			t.Errorf("MCP exposed forbidden arbitrary execution tool: %s", output.String())
		}
	}
	if len(surface.operations) != 1 || surface.operations[0].Name != "resume" || surface.operations[0].ID != "labor-1" {
		t.Errorf("operations = %#v", surface.operations)
	}
}

func TestServerReturnsProtocolAndToolErrorsWithoutLosingSurfaceOutcome(t *testing.T) {
	t.Parallel()

	surface := &fakeSurface{err: errors.New("blocked for operator")}
	input := strings.Join([]string{
		`not json`,
		`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":2,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"heracles_resume","arguments":{"id":"labor-1"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"heracles_shell","arguments":{}}}`,
	}, "\n") + "\n"
	var output bytes.Buffer
	if err := (mcp.Server{Surface: surface}).Serve(context.Background(), strings.NewReader(input), &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	for _, expected := range []string{`"code":-32700`, `"code":-32002`, `"isError":true`, "blocked for operator", `"code":-32602`} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("output does not contain %q: %s", expected, output.String())
		}
	}
}

func TestCLIAndMCPExposeEquivalentSharedOperationOutcome(t *testing.T) {
	t.Parallel()

	surface := &fakeSurface{}
	var cliOutput, cliError bytes.Buffer
	if exit := cli.RunWithOptions([]string{"resume", "labor-1", "--json"}, &cliOutput, &cliError, cli.Options{Control: surface}); exit != 0 {
		t.Fatalf("CLI exit = %d; stderr = %q", exit, cliError.String())
	}
	var mcpOutput bytes.Buffer
	input := "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":{}}\n{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"heracles_resume\",\"arguments\":{\"id\":\"labor-1\"}}}\n"
	if err := (mcp.Server{Surface: surface}).Serve(context.Background(), strings.NewReader(input), &mcpOutput); err != nil {
		t.Fatalf("MCP Serve() error = %v", err)
	}
	for _, output := range []string{cliOutput.String(), mcpOutput.String()} {
		for _, expected := range []string{"resume", "labor-1", "ok"} {
			if !strings.Contains(output, expected) {
				t.Errorf("equivalent output does not contain %q: %s", expected, output)
			}
		}
	}
}

type fakeSurface struct {
	operations []control.Operation
	err        error
}

func (surface *fakeSurface) Execute(_ context.Context, operation control.Operation) (control.Result, error) {
	surface.operations = append(surface.operations, operation)
	return control.Result{Operation: operation.Name, ID: operation.ID, Status: "ok"}, surface.err
}

func (surface *fakeSurface) Close() error { return nil }
