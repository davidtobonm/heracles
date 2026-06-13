package fakeexec_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/davidtobonm/heracles/internal/testutil/fakeexec"
)

func TestExecutableReturnsConfiguredOutputAndRecordsInvocation(t *testing.T) {
	t.Parallel()

	fake := fakeexec.New(t, fakeexec.Response{
		Stdout:   "agent result\n",
		Stderr:   "agent note\n",
		ExitCode: 7,
	})

	command := exec.Command(fake.Path, "--model", "test-model")
	command.Stdin = strings.NewReader("prompt text")
	stdout, err := command.Output()

	if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 7 {
		t.Fatalf("command error = %v, want exit code 7", err)
	}
	if string(stdout) != "agent result\n" {
		t.Fatalf("stdout = %q, want configured output", stdout)
	}

	invocation := fake.Invocation(t)
	if invocation.Args != "--model\ntest-model\n" {
		t.Errorf("args = %q, want recorded arguments", invocation.Args)
	}
	if invocation.Stdin != "prompt text" {
		t.Errorf("stdin = %q, want recorded prompt", invocation.Stdin)
	}
	if invocation.Stderr != "agent note\n" {
		t.Errorf("stderr = %q, want configured stderr", invocation.Stderr)
	}
}
