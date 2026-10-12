package mutagen

import "testing"

func TestSyncRunning(t *testing.T) {
	for status, want := range map[string]bool{
		"Watching for changes":                  true,
		"[Paused]":                              false,
		"":                                      false,
		"Halted due to one-sided root emptying": false,
	} {
		if got := SyncRunning(status); got != want {
			t.Errorf("SyncRunning(%q) = %v, want %v", status, got, want)
		}
	}
}

func TestHalted(t *testing.T) {
	if !Halted("Halted due to one-sided root emptying") {
		t.Fatal("expected halted")
	}
	if Halted("Watching for changes") {
		t.Fatal("watching is not halted")
	}
}

func TestParseSessionList(t *testing.T) {
	out := `Name: foo
Status: Watching for changes
Name: bar
Status: [Paused]
Name: baz
Status: Halted due to one-sided root emptying
`
	sessions := parseSessionList(out)
	if len(sessions) != 3 || sessions[0].Name != "foo" || sessions[1].Status != "[Paused]" {
		t.Fatalf("sessions = %#v", sessions)
	}
	if !Halted(sessions[2].Status) {
		t.Fatalf("baz status = %q", sessions[2].Status)
	}
}
