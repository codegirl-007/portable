package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/codegirl-007/portable/internal/config"
	"github.com/codegirl-007/portable/internal/remote"
)

func TestIsHexToken(t *testing.T) {
	if !isHexToken("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa") {
		t.Fatal("32 hex should pass")
	}
	if isHexToken("zz") || isHexToken("") || isHexToken("abc") {
		t.Fatal("expected reject")
	}
}

func TestParseTokenOutputIgnoresNoise(t *testing.T) {
	const tok = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	got := parseTokenOutput("sprite: waking\n" + tok + "\n")
	if got != tok {
		t.Fatalf("got %q", got)
	}
	if parseTokenOutput("hello") != "" {
		t.Fatal("expected empty")
	}
}

func TestPrintAccessNoQR(t *testing.T) {
	var buf bytes.Buffer
	u := "https://demo.sprites.app/?t=secret"
	if err := printAccess(&buf, u, true); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(buf.String()) != u {
		t.Fatalf("got %q", buf.String())
	}
}

func TestRemoteSessionCommandUsesAgent(t *testing.T) {
	defer func() { cfg = nil }()
	cfg = &config.Config{
		SetupVersion: 1,
		DefaultAgent: "opencode",
		Agents: []config.Agent{{
			Name: "opencode",
			Run:  "exec opencode",
		}},
	}
	got, err := remoteSessionCommand()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "exec opencode") {
		t.Fatalf("got %q", got)
	}
}

func TestRemoteSessionCommandLocalOnlyShell(t *testing.T) {
	defer func() { cfg = nil }()
	cfg = &config.Config{
		SetupVersion: 1,
		DefaultAgent: "cursor",
		Agents: []config.Agent{{
			Name:      "cursor",
			LocalOnly: true,
			Run:       "exec agent",
		}},
	}
	got, err := remoteSessionCommand()
	if err != nil {
		t.Fatal(err)
	}
	if got != "exec bash -l" {
		t.Fatalf("got %q", got)
	}
}

func TestRemoteCommandRegistered(t *testing.T) {
	var names []string
	for _, c := range rootCmd.Commands() {
		names = append(names, c.Name())
	}
	joined := strings.Join(names, " ")
	if !strings.Contains(joined, "remote") {
		t.Fatalf("commands = %s", joined)
	}
	if !strings.Contains(joined, "gateway") {
		t.Fatalf("hidden gateway missing from commands = %s", joined)
	}
}

func TestAgentWrapsTmux(t *testing.T) {
	s := remote.TmuxAttachOrCreate(remote.SessionName, "/home/sprite/app", "exec claude")
	if !strings.Contains(s, "tmux") || !strings.Contains(s, "exec claude") {
		t.Fatalf("%s", s)
	}
}

func TestDisableScriptDoesNotDestroy(t *testing.T) {
	s := remote.DisableScript()
	for _, bad := range []string{"destroy", "mutagen", "rm -rf"} {
		if strings.Contains(s, bad) {
			t.Fatalf("disable script contains %q: %s", bad, s)
		}
	}
	if !strings.Contains(s, "services delete "+remote.ServiceName) {
		t.Fatalf("%s", s)
	}
}
