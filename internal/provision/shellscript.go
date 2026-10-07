package provision

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	safePathSegment   = regexp.MustCompile(`^\$HOME(/[a-zA-Z0-9._+-]+)*$`)
	safeToolCommand   = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._+-]*$`)
)

// GuardedToolInstall wraps an install script so it runs only when commandName is not on PATH.
func GuardedToolInstall(commandName, installLine string) (string, error) {
	commandName = strings.TrimSpace(commandName)
	installLine = strings.TrimSpace(installLine)
	if installLine == "" {
		return "", fmt.Errorf("provision: empty tool install command")
	}
	if err := ValidateShellLines([]string{installLine}); err != nil {
		return "", err
	}
	if commandName == "" {
		return "", fmt.Errorf("provision: tool command name is required")
	}
	if !safeToolCommand.MatchString(commandName) {
		return "", fmt.Errorf("provision: invalid tool command name %q", commandName)
	}
	return fmt.Sprintf(`command -v %s >/dev/null 2>&1 || %s`, commandName, installLine), nil
}

// AppendPathSegments adds validated PATH prefixes for generated shell scripts.
func AppendPathSegments(base, extra []string) ([]string, error) {
	out := append([]string(nil), base...)
	for _, p := range extra {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if !safePathSegment.MatchString(p) {
			return nil, fmt.Errorf("provision: invalid PATH segment %q", p)
		}
		out = append(out, p)
	}
	return out, nil
}

// PathExportLine builds a validated export PATH= line for bash scripts.
func PathExportLine(extra []string) (string, error) {
	base := []string{`$HOME/.local/bin`, `$HOME/.bun/bin`, `$HOME/.deno/bin`, `$HOME/.opencode/bin`}
	bits, err := AppendPathSegments(base, extra)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`export PATH="%s:$PATH"`, strings.Join(bits, ":")), nil
}

// ValidateShellLines rejects commands that would break a generated script.
func ValidateShellLines(lines []string) error {
	for _, line := range lines {
		if strings.Contains(line, "\n") || strings.Contains(line, "\r") {
			return fmt.Errorf("provision: multiline shell command")
		}
	}
	return nil
}

func writeLines(writeln func(string), lines []string) error {
	if err := ValidateShellLines(lines); err != nil {
		return err
	}
	for _, line := range lines {
		writeln(line)
	}
	return nil
}
