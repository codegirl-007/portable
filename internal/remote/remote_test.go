package remote

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewToken(t *testing.T) {
	a, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 64 || len(b) != 64 || a == b {
		t.Fatalf("tokens %q %q", a, b)
	}
}

func TestAccessURL(t *testing.T) {
	got, err := AccessURL("https://demo-abc.sprites.app/", "deadbeef")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://demo-abc.sprites.app/#t=deadbeef" {
		t.Fatalf("got %q", got)
	}
	if strings.Contains(got, "?t=") {
		t.Fatal("token must not be a query parameter")
	}
	if _, err := AccessURL("", "x"); err == nil {
		t.Fatal("expected empty URL error")
	}
}

func TestParseSpriteHTTPJSON(t *testing.T) {
	info := ParseSpriteHTTP(`{"url":"https://name-xyz.sprites.app","url_settings":{"auth":"sprite"}}`)
	if info.URL != "https://name-xyz.sprites.app" || info.Auth != "sprite" {
		t.Fatalf("%+v", info)
	}
}

func TestParseSpriteHTTPText(t *testing.T) {
	info := ParseSpriteHTTP("URL: https://my-sprite-ab12.sprites.app\nAuth: public\n")
	if info.URL != "https://my-sprite-ab12.sprites.app" || info.Auth != "public" {
		t.Fatalf("%+v", info)
	}
	info = ParseSpriteHTTP("URL: https://my-sprite-ab12.sprites.app\n")
	if info.URL != "https://my-sprite-ab12.sprites.app" {
		t.Fatalf("%+v", info)
	}
	if info.Auth != "" {
		t.Fatalf("substring sprite in the hostname must not set auth: %+v", info)
	}
}

func TestWithShellFallback(t *testing.T) {
	got := WithShellFallback("export PATH=x; exec opencode")
	if got != "export PATH=x; opencode; exec bash -l" {
		t.Fatalf("got %q", got)
	}
	if again := WithShellFallback(got); again != got {
		t.Fatalf("not idempotent: %q", again)
	}
	if got := WithShellFallback("exec bash -l"); got != "exec bash -l" {
		t.Fatalf("bash = %q", got)
	}
}

func TestLinuxGOARCH(t *testing.T) {
	got, err := LinuxGOARCH("x86_64\n")
	if err != nil || got != "amd64" {
		t.Fatalf("%q %v", got, err)
	}
	got, err = LinuxGOARCH("aarch64")
	if err != nil || got != "arm64" {
		t.Fatalf("%q %v", got, err)
	}
	if _, err := LinuxGOARCH("ppc64"); err == nil {
		t.Fatal("expected error")
	}
}

func TestTmuxAttachOrCreateQuotes(t *testing.T) {
	s := TmuxAttachOrCreate("portable", "/home/sprite/app", "export PATH='$HOME/bin'; exec opencode")
	for _, want := range []string{
		"command -v tmux",
		InstallTmuxShell(),
		"tmux -u attach-session -t 'portable'",
		"tmux -u new-session -s 'portable' -c '/home/sprite/app'",
		"opencode; exec bash -l",
		`'\''`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in %s", want, s)
		}
	}
}

func TestTmuxFallsBackWithoutTmux(t *testing.T) {
	s := TmuxAttachOrCreate("portable", "/tmp", "exec claude")
	if !strings.Contains(s, "exec sh -c 'claude; exec bash -l'") {
		t.Fatalf("fallback missing: %s", s)
	}
}

func TestEnableScript(t *testing.T) {
	s, err := EnableScript(EnableRequest{
		Token:   "abc123def456abc123def456abc123def456abc123def456abc123def456abcd",
		Dir:     "/home/sprite/app",
		Command: "export PATH=x; exec opencode",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"tmux",
		"chmod 600",
		TokenRel,
		SessionRel,
		"sprite-env services delete " + ServiceName,
		"sprite-env services create " + ServiceName,
		"--http-port 7681",
		"gateway,--listen,:7681",
		BinaryPath,
		"abc123def456abc123def456abc123def456abc123def456abc123def456abcd",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("enable script missing %q", want)
		}
	}
	if strings.Contains(s, "/home/sprite/app/.config") {
		t.Fatal("token must not live in the synced project directory")
	}
}

func TestEnableScriptPublicHost(t *testing.T) {
	s, err := EnableScript(EnableRequest{
		Token:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Dir:        "/home/sprite/app",
		Command:    "exec opencode",
		PublicHost: "demo-abc.sprites.app",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s, "gateway,--listen,:7681,--public-host,demo-abc.sprites.app") {
		t.Fatalf("missing public-host in %s", s)
	}
}

func TestEnableScriptRequiresToken(t *testing.T) {
	if _, err := EnableScript(EnableRequest{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestEnableScriptSyntax(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not available")
	}
	s, err := EnableScript(EnableRequest{
		Token:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Dir:     "/home/sprite/it's app",
		Command: "echo hi",
	})
	if err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), "enable.sh")
	if err := os.WriteFile(f, []byte(s+"\n"+DisableScript()), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(bash, "-n", f).CombinedOutput(); err != nil {
		t.Fatalf("syntax: %v\n%s", err, out)
	}
}

func TestTokenAndSessionPathsAreHomeNotProject(t *testing.T) {
	if !strings.HasPrefix(TokenFile(), "/home/sprite/.config/") {
		t.Fatalf("TokenFile = %s", TokenFile())
	}
	if strings.Contains(TokenFile(), "RemoteDir") {
		t.Fatal("token path must not use the project dir")
	}
}

func TestFindModuleRoot(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := FindModuleRoot(wd)
	if root == "" {
		t.Fatal("expected module root")
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatal(err)
	}
}

func TestLinuxBinarySelfOnLinux(t *testing.T) {
	if os.Getenv("GOOS") == "windows" {
		t.Skip()
	}
	p, cleanup, err := LinuxBinary("amd64", "dev", "")
	if err != nil {
		t.Skip(err)
	}
	defer cleanup()
	if p == "" {
		t.Fatal("empty path")
	}
}

func TestWriteQR(t *testing.T) {
	var b strings.Builder
	if err := WriteQR(&b, "https://example.sprites.app/#t=abc"); err != nil {
		t.Fatal(err)
	}
	if b.Len() < 50 {
		t.Fatalf("qr too small: %q", b.String())
	}
}

func TestEncodeSession(t *testing.T) {
	b, err := EncodeSession("/home/sprite/app", "exec opencode")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"/home/sprite/app"`) {
		t.Fatalf("%s", b)
	}
}
