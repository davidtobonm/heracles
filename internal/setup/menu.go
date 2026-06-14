// Package setup implements the interactive `heracles init` flow described by
// ADR 0021: Fast and Complete Setup, Reconfigure, and Repair.
package setup

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// ErrCancelled indicates the user explicitly cancelled an interactive prompt.
var ErrCancelled = errors.New("setup cancelled")

// IO is the terminal abstraction shared by every interactive prompt.
type IO struct {
	In  *bufio.Reader
	Out io.Writer

	// Terminal returns the input file descriptor and whether it is an
	// interactive terminal. A nil Terminal, or one returning ok=false,
	// selects the numbered-list fallback used by tests, CI, and piped input.
	Terminal func() (fd int, ok bool)
}

// NewIO builds an IO that detects a real terminal on in.
func NewIO(in io.Reader, out io.Writer, terminal func() (int, bool)) IO {
	return IO{In: bufio.NewReader(in), Out: out, Terminal: terminal}
}

type key int

const (
	keyNone key = iota
	keyUp
	keyDown
	keyEnter
	keyQuit
)

// parseKey interprets a raw terminal byte sequence as a navigation key.
// It is pure so menu navigation is unit-testable without a real TTY.
func parseKey(buf []byte) key {
	switch len(buf) {
	case 1:
		switch buf[0] {
		case '\r', '\n':
			return keyEnter
		case 'q', 3: // 3 == Ctrl-C
			return keyQuit
		}
	case 3:
		if buf[0] == 0x1b && buf[1] == '[' {
			switch buf[2] {
			case 'A':
				return keyUp
			case 'B':
				return keyDown
			}
		}
	}
	return keyNone
}

// SelectOption prompts for one of options, returning its index. current is
// the pre-selected (and default) index, clamped into range.
func SelectOption(io IO, title string, options []string, current int) (int, error) {
	if len(options) == 0 {
		return 0, errors.New("setup: no options to select")
	}
	if current < 0 || current >= len(options) {
		current = 0
	}
	if io.Terminal != nil {
		if fd, ok := io.Terminal(); ok {
			return selectInteractive(io, fd, title, options, current)
		}
	}
	return selectNumbered(io, title, options, current)
}

func selectNumbered(io IO, title string, options []string, current int) (int, error) {
	fmt.Fprintln(io.Out, title)
	for index, option := range options {
		marker := ""
		if index == current {
			marker = " (current)"
		}
		fmt.Fprintf(io.Out, "  %d) %s%s\n", index+1, option, marker)
	}
	for {
		fmt.Fprintf(io.Out, "Select 1-%d [%d]: ", len(options), current+1)
		line, err := readLine(io.In)
		if err != nil {
			return current, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			return current, nil
		}
		index, err := strconv.Atoi(line)
		if err != nil || index < 1 || index > len(options) {
			fmt.Fprintf(io.Out, "enter a number between 1 and %d\n", len(options))
			continue
		}
		return index - 1, nil
	}
}

func selectInteractive(io IO, fd int, title string, options []string, current int) (int, error) {
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return selectNumbered(io, title, options, current)
	}
	defer term.Restore(fd, oldState)

	selected := current
	draw := func(first bool) {
		if !first {
			fmt.Fprintf(io.Out, "\x1b[%dA", len(options)+1)
		}
		fmt.Fprintf(io.Out, "\r\x1b[2K%s\r\n", title)
		for index, option := range options {
			marker := "  "
			if index == selected {
				marker = "> "
			}
			fmt.Fprintf(io.Out, "\r\x1b[2K%s%s\r\n", marker, option)
		}
	}
	draw(true)
	for {
		buf, err := readKeySequence(io.In)
		if err != nil {
			return current, err
		}
		switch parseKey(buf) {
		case keyUp:
			selected = (selected - 1 + len(options)) % len(options)
			draw(false)
		case keyDown:
			selected = (selected + 1) % len(options)
			draw(false)
		case keyEnter:
			return selected, nil
		case keyQuit:
			return current, ErrCancelled
		}
	}
}

// readKeySequence reads one key press, expanding `\x1b[` escape sequences to three bytes.
func readKeySequence(reader *bufio.Reader) ([]byte, error) {
	first, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	if first != 0x1b {
		return []byte{first}, nil
	}
	second, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	third, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	return []byte{first, second, third}, nil
}

// Confirm asks a yes/no question, returning defaultYes when the user presses Enter.
func Confirm(io IO, question string, defaultYes bool) (bool, error) {
	prompt := "[y/N]"
	if defaultYes {
		prompt = "[Y/n]"
	}
	for {
		fmt.Fprintf(io.Out, "%s %s ", question, prompt)
		line, err := readLine(io.In)
		if err != nil {
			return defaultYes, err
		}
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "":
			return defaultYes, nil
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		}
		fmt.Fprintln(io.Out, "enter y or n")
	}
}

// Text asks a free-text question, returning defaultValue when the user presses Enter.
func Text(io IO, question, defaultValue string) (string, error) {
	if defaultValue != "" {
		fmt.Fprintf(io.Out, "%s (default: %s): ", question, defaultValue)
	} else {
		fmt.Fprintf(io.Out, "%s: ", question)
	}
	line, err := readLine(io.In)
	if err != nil {
		return defaultValue, err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultValue, nil
	}
	return line, nil
}

func readLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		if line == "" {
			return "", err
		}
		if !errors.Is(err, io.EOF) {
			return "", err
		}
	}
	return strings.TrimRight(line, "\r\n"), nil
}
