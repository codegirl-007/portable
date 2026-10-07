package opencode

import "testing"

func TestKeyFromValue(t *testing.T) {
	if got := keyFromValue(`{"type":"api","key":"abc123"}`); got != "abc123" {
		t.Fatalf("got %q", got)
	}
	if got := keyFromValue("not json"); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestKeyScanRegex(t *testing.T) {
	const key = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	m := keyRe.FindStringSubmatch(`prefix{"type":"api","key":"` + key + `"}suffix`)
	if m == nil || m[1] != key {
		t.Fatalf("scan failed: %v", m)
	}
}
