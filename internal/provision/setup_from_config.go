package provision

import (
	"strings"

	"github.com/codegirl-007/portable/internal/config"
	"github.com/codegirl-007/portable/internal/setup"
)

// SetupRequestFromConfig builds a provision request from global config and
// resolved credential file contents.
type SetupRequestFromConfig struct {
	PublicKey       string
	RemoteDir       string
	CredentialFiles []CredentialFile
	OpenCodeModel   string
}

// NewSetupRequest merges portable config with credential material for the setup script.
func NewSetupRequest(cfg *config.Config, base SetupRequestFromConfig) SetupRequest {
	req := SetupRequest{
		PublicKey: base.PublicKey,
		RemoteDir: base.RemoteDir,
		Tools:     cfg.Tools,
		EnvFiles:  append([]string(nil), cfg.EnvFiles...),
		Nvim:      cfg.Provision.Nvim,
		Gh:        cfg.Provision.Gh,
		Ripgrep:   cfg.Provision.Ripgrep,
		TreeSitter: cfg.Provision.TreeSitter,
		NvimBootstrapDot: cfg.Provision.Nvim && hasDotfile(cfg.Dotfiles, ".config/nvim"),
	}

	agent, ok := cfg.DefaultAgentConfig()
	if ok && !agent.LocalOnly {
		req.AgentInstall = append(req.AgentInstall, agent.Install...)
	}

	for _, cf := range base.CredentialFiles {
		req.CredentialFiles = append(req.CredentialFiles, cf)
	}

	if agent, ok := cfg.DefaultAgentConfig(); ok && agent.Credential == "opencode" {
		req.OpenCodeModel = setup.OpenCodeModel()
	}

	return req
}

func hasDotfile(dotfiles []string, want string) bool {
	for _, d := range dotfiles {
		if d == want || strings.HasPrefix(d, want+"/") {
			return true
		}
	}
	return false
}
