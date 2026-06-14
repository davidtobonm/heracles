package setup

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func TestParseKeyRecognizesArrowsAndControlKeys(t *testing.T) {
	t.Parallel()

	cases := map[string]key{
		"\x1b[A": keyUp,
		"\x1b[B": keyDown,
		"\r":     keyEnter,
		"\n":     keyEnter,
		"q":      keyQuit,
		"\x03":   keyQuit,
		"\x1b[C": keyNone,
		"x":      keyNone,
	}
	for input, want := range cases {
		if got := parseKey([]byte(input)); got != want {
			t.Errorf("parseKey(%q) = %v, want %v", input, got, want)
		}
	}
}

func newIO(input string) (IO, *bytes.Buffer) {
	var out bytes.Buffer
	return IO{In: bufio.NewReader(strings.NewReader(input)), Out: &out}, &out
}

func TestSelectOptionNumberedFallbackParsesChoice(t *testing.T) {
	t.Parallel()

	io, out := newIO("2\n")
	index, err := SelectOption(io, "Pick one", []string{"alpha", "beta", "gamma"}, 0)
	if err != nil {
		t.Fatalf("SelectOption() error = %v", err)
	}
	if index != 1 {
		t.Errorf("SelectOption() = %d, want 1", index)
	}
	if !strings.Contains(out.String(), "1) alpha (current)") {
		t.Errorf("output = %q, want current marker on default option", out.String())
	}
}

func TestSelectOptionNumberedFallbackEmptyKeepsCurrent(t *testing.T) {
	t.Parallel()

	io, _ := newIO("\n")
	index, err := SelectOption(io, "Pick one", []string{"alpha", "beta"}, 1)
	if err != nil {
		t.Fatalf("SelectOption() error = %v", err)
	}
	if index != 1 {
		t.Errorf("SelectOption() = %d, want 1 (unchanged current)", index)
	}
}

func TestSelectOptionNumberedFallbackRejectsOutOfRange(t *testing.T) {
	t.Parallel()

	io, out := newIO("9\n1\n")
	index, err := SelectOption(io, "Pick one", []string{"alpha", "beta"}, 0)
	if err != nil {
		t.Fatalf("SelectOption() error = %v", err)
	}
	if index != 0 {
		t.Errorf("SelectOption() = %d, want 0", index)
	}
	if !strings.Contains(out.String(), "enter a number between 1 and 2") {
		t.Errorf("output = %q, want range hint after invalid choice", out.String())
	}
}

func TestConfirmDefaultsAndParsesAnswers(t *testing.T) {
	t.Parallel()

	io, _ := newIO("\n")
	got, err := Confirm(io, "Continue?", true)
	if err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}
	if !got {
		t.Errorf("Confirm() = %v, want true (default)", got)
	}

	io, _ = newIO("no\n")
	got, err = Confirm(io, "Continue?", true)
	if err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}
	if got {
		t.Errorf("Confirm() = %v, want false", got)
	}
}

func TestConfirmReprompts(t *testing.T) {
	t.Parallel()

	io, out := newIO("maybe\ny\n")
	got, err := Confirm(io, "Continue?", false)
	if err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}
	if !got {
		t.Errorf("Confirm() = %v, want true", got)
	}
	if !strings.Contains(out.String(), "enter y or n") {
		t.Errorf("output = %q, want reprompt hint", out.String())
	}
}

func TestTextDefaultsWhenEmpty(t *testing.T) {
	t.Parallel()

	io, _ := newIO("\n")
	got, err := Text(io, "Name?", "default-value")
	if err != nil {
		t.Fatalf("Text() error = %v", err)
	}
	if got != "default-value" {
		t.Errorf("Text() = %q, want default value", got)
	}

	io, _ = newIO("custom\n")
	got, err = Text(io, "Name?", "default-value")
	if err != nil {
		t.Fatalf("Text() error = %v", err)
	}
	if got != "custom" {
		t.Errorf("Text() = %q, want %q", got, "custom")
	}
}

func TestSelectOptionWithoutTrailingNewlineUsesAvailableInput(t *testing.T) {
	t.Parallel()

	io, _ := newIO("2")
	index, err := SelectOption(io, "Pick one", []string{"alpha", "beta"}, 0)
	if err != nil {
		t.Fatalf("SelectOption() error = %v", err)
	}
	if index != 1 {
		t.Errorf("SelectOption() = %d, want 1", index)
	}
}
