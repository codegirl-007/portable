// Package ui provides tiny terminal output helpers.
package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// Step prints a top-level progress line.
func Step(format string, args ...any) { fmt.Printf("==> "+format+"\n", args...) }

// Infof prints an indented detail line.
func Infof(format string, args ...any) { fmt.Printf("    "+format+"\n", args...) }

// Successf prints a success line.
func Successf(format string, args ...any) { fmt.Printf("✓ "+format+"\n", args...) }

// Warnf prints a warning to stderr.
func Warnf(format string, args ...any) { fmt.Fprintf(os.Stderr, "warning: "+format+"\n", args...) }

// IsTerminal reports whether stdin is an interactive terminal.
func IsTerminal() bool { return term.IsTerminal(int(os.Stdin.Fd())) }

// Confirm asks a yes/no question on stdin. It returns false if stdin is not a
// terminal.
func Confirm(prompt string) bool {
	if !IsTerminal() {
		return false
	}
	fmt.Printf("%s [y/N] ", prompt)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}
