package mutagen

import "testing"

func TestSyncRunning(t *testing.T) {
	for status, want := range map[string]bool{
		"Watching for changes": true,
		"[Paused]":             false,
		"":                     false,
	} {
		if got := SyncRunning(status); got != want {
			t.Errorf("SyncRunning(%q) = %v, want %v", status, got, want)
		}
	}
}

func TestParseSessionList(t *testing.T) {
	out := `Name: foo
Status: Watching for changes
Name: bar
Status: [Paused]
`
	sessions := parseSessionList(out)
	if len(sessions) != 2 || sessions[0].Name != "foo" || sessions[1].Status != "[Paused]" {
		t.Fatalf("sessions = %#v", sessions)
	}
}
