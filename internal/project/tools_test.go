package project

import (
	"strings"
	"testing"
)

func TestDepsPlanToolsGuarded(t *testing.T) {
	plan, err := DepsPlanFromConfig(Config{
		Tools: []ToolEntry{{
			Command: "bun",
			Install: "curl -fsSL https://bun.sh/install | bash",
		}},
		Install: []string{"bun install"},
	}, func() ([]string, []string) {
		return []string{"detect-ensure"}, []string{"detect-install"}
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Ensure) != 1 {
		t.Fatalf("ensure = %v", plan.Ensure)
	}
	if !strings.HasPrefix(plan.Ensure[0], "command -v bun ") {
		t.Fatalf("ensure[0] = %q", plan.Ensure[0])
	}
	if !strings.Contains(plan.Ensure[0], "curl -fsSL https://bun.sh/install | bash") {
		t.Fatalf("ensure[0] = %q", plan.Ensure[0])
	}
}

func TestDepsPlanToolsThenEnsure(t *testing.T) {
	plan, err := DepsPlanFromConfig(Config{
		Tools: []ToolEntry{{
			Command: "node",
			Install: "apt-get install -y nodejs",
		}},
		Ensure:  []string{"corepack enable"},
		Install: []string{"pnpm install"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Ensure) != 2 || plan.Ensure[1] != "corepack enable" {
		t.Fatalf("ensure = %v", plan.Ensure)
	}
}

func TestDepsPlanProjectOverridesDetect(t *testing.T) {
	plan, err := DepsPlanFromConfig(Config{
		Tools: []ToolEntry{{
			Command: "bun",
			Install: "curl -fsSL https://bun.sh/install | bash",
		}},
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
