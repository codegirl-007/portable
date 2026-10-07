package provision

import "testing"

func TestGuardedToolInstall(t *testing.T) {
	line, err := GuardedToolInstall("bun", "curl -fsSL https://bun.sh/install | bash")
	if err != nil {
		t.Fatal(err)
	}
	want := `command -v bun >/dev/null 2>&1 || curl -fsSL https://bun.sh/install | bash`
	if line != want {
		t.Fatalf("got %q", line)
	}
}

func TestGuardedToolInstallRejectsBadName(t *testing.T) {
	if _, err := GuardedToolInstall("bun; rm", "true"); err == nil {
		t.Fatal("expected error")
	}
}
