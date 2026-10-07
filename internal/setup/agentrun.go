package setup

import "fmt"

// RunCommand returns the fixed remote shell prefix for a built-in agent id.
func RunCommand(agentID string) (string, error) {
	for _, c := range Choices() {
		if c.ID != agentID {
			continue
		}
		a := c.Build()
		if a.LocalOnly || a.Run == "" {
			return "", fmt.Errorf("agent %q has no remote run command", agentID)
		}
		return a.Run, nil
	}
	return "", fmt.Errorf("unknown agent %q", agentID)
}
