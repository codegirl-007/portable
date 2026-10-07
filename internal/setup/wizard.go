package setup

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/codegirl-007/portable/internal/ui"
)

// RunInteractive collects setup answers on stdin.
func RunInteractive(force bool) (*WizardResult, error) {
	if !ui.IsTerminal() {
		return nil, fmt.Errorf("setup requires an interactive terminal (or use flags: portable setup --help)")
	}
	if !force && configExists() {
		fmt.Printf("Existing config at %s will be replaced (saved as config.toml.bak).\n", globalPathHint())
		if !ui.Confirm("Continue?") {
			return nil, fmt.Errorf("setup cancelled")
		}
	}

	ui.Step("Choose your coding agent")
	fmt.Println("This is used for `portable agent` on every project.")
	choices := Choices()
	for i, c := range choices {
		fmt.Printf("  %d) %s — %s\n", i+1, c.Label, c.Description)
	}
	fmt.Println()
	agentIdx, err := readIntChoice("Enter number", 1, len(choices))
	if err != nil {
		return nil, err
	}
	agentID := choices[agentIdx-1].ID

	ui.Step("Your dev tooling on the workspace (optional)")
	fmt.Println("These are for you on every project — not per-repo language choices.")
	nvim := ui.Confirm("Install Neovim on the workspace?")
	gh := ui.Confirm("Install GitHub CLI (gh) on the workspace?")

	dotfiles, err := readDotfilesInteractive(nvim)
	if err != nil {
		return nil, err
	}

	var tools []string
	if ui.Confirm("Add custom one-time workspace bootstrap commands (advanced)?") {
		fmt.Println("Shell commands run once when a workspace is first created. Empty line to finish:")
		tools = readLinesUntilEmpty()
	}

	return &WizardResult{
		AgentID:  agentID,
		Nvim:     nvim,
		Gh:       gh,
		Dotfiles: dotfiles,
		Tools:    tools,
	}, nil
}

// WizardResult holds answers from the setup wizard.
type WizardResult struct {
	AgentID  string
	Nvim     bool
	Gh       bool
	Dotfiles []string
	Tools    []string
}

func readIntChoice(prompt string, min, max int) (int, error) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("%s [%d-%d]: ", prompt, min, max)
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		n, err := strconv.Atoi(line)
		if err != nil || n < min || n > max {
			fmt.Printf("    enter a number between %d and %d\n", min, max)
			continue
		}
		return n, nil
	}
}

func readDotfilesInteractive(nvimOnWorkspace bool) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	ui.Step("Dotfiles")
	fmt.Println("Home-relative paths copied once at provision. Missing paths on your laptop are skipped.")
	opts := DotfileSuggestions(nvimOnWorkspace)
	for i, o := range opts {
		tag := "not found locally"
		if homePathExists(home, o.Path) {
			tag = "found on this machine"
		}
		fmt.Printf("  %d) %-22s %s (%s)\n", i+1, o.Path, o.Hint, tag)
	}
	fmt.Println()
	fmt.Println("Suggested default: copy every path above that exists on this machine.")
	fmt.Println("Or enter numbers to copy (e.g. 1 2), a comma-separated list, 'all' for every suggestion, or 'none'.")
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Dotfiles [default]: ")
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))

	switch line {
	case "", "default", "d", "y", "yes":
		return FilterExisting(home, CandidatePaths(opts)), nil
	case "none", "n", "no":
		return nil, nil
	case "all":
		return CandidatePaths(opts), nil
	}

	if nums := parseNumberList(line, len(opts)); nums != nil {
		var out []string
		for _, i := range nums {
			out = append(out, opts[i-1].Path)
		}
		return out, nil
	}

	if strings.Contains(line, ",") {
		fields := strings.Split(line, ",")
		allNum := true
		var nums []int
		for _, f := range fields {
			f = strings.TrimSpace(f)
			n, err := strconv.Atoi(f)
			if err != nil || n < 1 || n > len(opts) {
				allNum = false
				break
			}
			nums = append(nums, n)
		}
		if allNum && len(nums) > 0 {
			var out []string
			for _, i := range nums {
				out = append(out, opts[i-1].Path)
			}
			return out, nil
		}
	}

	var out []string
	for _, p := range strings.Split(line, ",") {
		p = strings.TrimSpace(p)
		p = strings.TrimPrefix(p, "~/")
		if p != "" {
			out = append(out, p)
		}
	}
	return out, nil
}

func parseNumberList(line string, max int) []int {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return nil
	}
	var nums []int
	for _, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil || n < 1 || n > max {
			return nil
		}
		nums = append(nums, n)
	}
	return nums
}

func readLinesUntilEmpty() []string {
	reader := bufio.NewReader(os.Stdin)
	var out []string
	for {
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		out = append(out, line)
	}
	return out
}

func configExists() bool {
	_, err := os.Stat(globalPath())
	return err == nil
}

func globalPath() string {
	// avoid import cycle with config in tests — duplicate minimal path
	home, _ := os.UserHomeDir()
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return d + "/portable/config.toml"
	}
	return home + "/.config/portable/config.toml"
}

func globalPathHint() string {
	return globalPath()
}
