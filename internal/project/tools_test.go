package project

import "testing"

func TestEnsureCommandsBun(t *testing.T) {
	cmds, err := EnsureCommands([]string{"bun"})
	if err != nil || len(cmds) != 1 {
		t.Fatalf("EnsureCommands(bun) = %v, %v", cmds, err)
	}
}

func TestEnsureCommandsUnknown(t *testing.T) {
	if _, err := EnsureCommands([]string{"fortran"}); err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestDepsPlanProjectOverridesDetect(t *testing.T) {
	plan, err := DepsPlanFromConfig(Config{
		Tools:   []string{"bun"},
		Install: []string{"bun install"},
	}, func() ([]string, []string) {
		return []string{"detect-ensure"}, []string{"detect-install"}
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.AutoLangInstall {
		t.Fatal("expected no auto lang install when project defined")
	}
	if len(plan.Install) != 1 || plan.Install[0] != "bun install" {
		t.Fatalf("install = %v", plan.Install)
	}
}
