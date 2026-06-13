// Package mcp exposes the Heracles Control Surface over MCP stdio.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/davidtobonm/heracles/internal/control"
)

const ProtocolVersion = "2025-11-25"

// Server is a newline-delimited JSON-RPC MCP server.
type Server struct {
	Surface control.Surface
	Version string
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type toolCall struct {
	Name      string            `json:"name"`
	Arguments control.Operation `json:"arguments"`
}

// Serve processes MCP messages until EOF or cancellation.
func (server Server) Serve(ctx context.Context, input io.Reader, output io.Writer) error {
	if server.Surface == nil {
		return errors.New("MCP server requires a Control Surface")
	}
	encoder := json.NewEncoder(output)
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)
	initialized := false
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		line := scanner.Bytes()
		var message request
		if err := json.Unmarshal(line, &message); err != nil {
			if err := encoder.Encode(response{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "Parse error"}}); err != nil {
				return err
			}
			continue
		}
		if len(message.ID) == 0 {
			if message.Method == "notifications/initialized" {
				initialized = true
			}
			continue
		}
		reply := response{JSONRPC: "2.0", ID: message.ID}
		switch {
		case message.JSONRPC != "2.0":
			reply.Error = &rpcError{Code: -32600, Message: "Invalid Request"}
		case message.Method == "initialize":
			initialized = true
			reply.Result = map[string]any{
				"protocolVersion": ProtocolVersion,
				"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
				"serverInfo":      map[string]string{"name": "heracles", "version": server.Version},
				"instructions":    "Use named high-level Heracles tools. No arbitrary shell execution is exposed.",
			}
		case !initialized:
			reply.Error = &rpcError{Code: -32002, Message: "Server not initialized"}
		case message.Method == "ping":
			reply.Result = map[string]any{}
		case message.Method == "tools/list":
			reply.Result = map[string]any{"tools": tools()}
		case message.Method == "tools/call":
			result, rpcErr := server.callTool(ctx, message.Params)
			reply.Result, reply.Error = result, rpcErr
		default:
			reply.Error = &rpcError{Code: -32601, Message: "Method not found"}
		}
		if err := encoder.Encode(reply); err != nil {
			return fmt.Errorf("write MCP response: %w", err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read MCP stdio: %w", err)
	}
	return nil
}

func (server Server) callTool(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var call toolCall
	if err := json.Unmarshal(params, &call); err != nil {
		return nil, &rpcError{Code: -32602, Message: "Invalid tools/call params", Data: err.Error()}
	}
	name, exists := toolOperations()[call.Name]
	if !exists {
		return nil, &rpcError{Code: -32602, Message: "Unknown Heracles tool", Data: call.Name}
	}
	call.Arguments.Name = name
	result, err := server.Surface.Execute(ctx, call.Arguments)
	structured := map[string]any{"result": result}
	isError := false
	if err != nil {
		structured["error"] = err.Error()
		isError = true
	}
	contents, marshalErr := json.Marshal(structured)
	if marshalErr != nil {
		return nil, &rpcError{Code: -32603, Message: "Encode tool result", Data: marshalErr.Error()}
	}
	return map[string]any{
		"content":           []map[string]string{{"type": "text", "text": string(contents)}},
		"structuredContent": structured,
		"isError":           isError,
	}, nil
}

func toolOperations() map[string]string {
	return map[string]string{
		"heracles_init": "init", "heracles_doctor": "doctor",
		"heracles_plan": "plan", "heracles_issues": "issues", "heracles_run": "run", "heracles_labor": "labor",
		"heracles_approve": "approve", "heracles_reject": "reject", "heracles_retry": "retry",
		"heracles_resume": "resume", "heracles_cancel": "cancel", "heracles_list": "list", "heracles_inspect": "inspect",
	}
}

func tools() []map[string]any {
	names := []string{
		"heracles_init", "heracles_doctor", "heracles_plan", "heracles_issues", "heracles_run", "heracles_labor",
		"heracles_approve", "heracles_reject", "heracles_retry", "heracles_resume", "heracles_cancel", "heracles_list", "heracles_inspect",
	}
	tools := make([]map[string]any, 0, len(names))
	for _, name := range names {
		tools = append(tools, map[string]any{
			"name":        name,
			"description": strings.ReplaceAll(strings.TrimPrefix(name, "heracles_"), "_", " ") + " through Heracles policy",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"kind": map[string]string{"type": "string"}, "id": map[string]string{"type": "string"},
					"problem": map[string]string{"type": "string"}, "prd": map[string]string{"type": "string"},
					"decision": map[string]string{"type": "string"}, "reason": map[string]string{"type": "string"},
					"tracker":      map[string]string{"type": "string"},
					"repositories": map[string]any{"type": "array", "items": map[string]string{"type": "string"}},
				},
				"additionalProperties": false,
			},
			"annotations": map[string]any{"readOnlyHint": name == "heracles_list" || name == "heracles_inspect" || name == "heracles_doctor"},
		})
	}
	return tools
}
