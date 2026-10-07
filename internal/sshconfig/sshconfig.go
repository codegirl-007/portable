// Package sshconfig manages a marked Host block in ~/.ssh/config so Mutagen
// (and ssh) can reach a Sprite through the sprite CLI's ProxyCommand.
package sshconfig

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Alias returns the SSH host alias for a sprite.
func Alias(sprite string) string { return "portable-" + sprite }

func begin(alias string) string { return "# >>> portable: " + alias + " >>>" }
func end(alias string) string   { return "# <<< portable: " + alias + " <<<" }

func path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ssh", "config"), nil
}

// Ensure adds (or refreshes) the managed Host block for a sprite.
func Ensure(sprite string) error {
	if err := validate(sprite); err != nil {
		return err
	}
	p, err := path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, _ := os.ReadFile(p)
	s := removeBlock(string(b), Alias(sprite))
	block := fmt.Sprintf("%s\nHost %s\n  HostName %s\n  User sprite\n  StrictHostKeyChecking no\n  UserKnownHostsFile /dev/null\n  ProxyCommand sprite proxy -s %s -W 22\n%s\n",
		begin(Alias(sprite)), Alias(sprite), sprite, sprite, end(Alias(sprite)))
	s = strings.TrimRight(s, "\n") + "\n\n" + block
	return os.WriteFile(p, []byte(s), 0o600)
}

// Remove deletes the managed Host block for a sprite.
func Remove(sprite string) error {
	if err := validate(sprite); err != nil {
		return nil
	}
	p, err := path()
	if err != nil {
		return err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	s := strings.TrimRight(removeBlock(string(b), Alias(sprite)), "\n") + "\n"
	if err := os.WriteFile(p, []byte(s), 0o600); err != nil {
		return err
	}
	// Best effort: drop any host key recorded for this alias.
	_ = exec.Command("ssh-keygen", "-R", Alias(sprite)).Run()
	return nil
}

func removeBlock(s, alias string) string {
	re := regexp.MustCompile(`(?ms)\n?# >>> portable: ` + regexp.QuoteMeta(alias) +
		` >>>.*?# <<< portable: ` + regexp.QuoteMeta(alias) + ` <<<\n?`)
	return re.ReplaceAllString(s, "\n")
}

func validate(sprite string) error {
	if strings.TrimSpace(sprite) == "" {
		return fmt.Errorf("sshconfig: empty sprite name")
	}
	if strings.ContainsAny(sprite, " \t\r\n") {
		return fmt.Errorf("sshconfig: invalid sprite name %q", sprite)
	}
	return nil
}
