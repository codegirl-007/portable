package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/codegirl-007/portable/internal/gateway"
)

var gatewayFlags struct {
	listen     string
	tokenFile  string
	session    string
	dir        string
	command    string
	publicHost string
}

var gatewayCmd = &cobra.Command{
	Use:    "gateway",
	Short:  "Run the remote web terminal (on the Sprite)",
	Hidden: true,
	RunE:   runGateway,
}

func init() {
	gatewayCmd.Flags().StringVar(&gatewayFlags.listen, "listen", ":7681", "listen address")
	gatewayCmd.Flags().StringVar(&gatewayFlags.tokenFile, "token-file", "", "token file (default ~/.config/portable/remote.token)")
	gatewayCmd.Flags().StringVar(&gatewayFlags.session, "session", "portable", "tmux session name")
	gatewayCmd.Flags().StringVar(&gatewayFlags.dir, "dir", "", "working directory for a new tmux session")
	gatewayCmd.Flags().StringVar(&gatewayFlags.command, "command", "", "shell command for a new tmux session")
	gatewayCmd.Flags().StringVar(&gatewayFlags.publicHost, "public-host", "", "public hostname (sprites.app) for Origin and Secure cookies")
}

func runGateway(_ *cobra.Command, _ []string) error {
	token, err := loadGatewayToken()
	if err != nil {
		return err
	}
	dir := gatewayFlags.dir
	command := gatewayFlags.command
	if dir == "" && command == "" {
		if d, c, err := gateway.LoadSessionFile(gateway.DefaultSessionFile()); err == nil {
			dir, command = d, c
		}
	}
	srv := &gateway.Server{
		Token:        token,
		Listen:       gatewayFlags.listen,
		Session:      gatewayFlags.session,
		Dir:          dir,
		ShellCommand: command,
		PublicHost:   gatewayFlags.publicHost,
	}
	return srv.Run(bg())
}

func loadGatewayToken() (string, error) {
	path := gatewayFlags.tokenFile
	if path == "" {
		path = gateway.DefaultTokenFile()
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read remote token: %w", err)
	}
	t := strings.TrimSpace(string(b))
	if t == "" {
		return "", fmt.Errorf("remote token file %s is empty", path)
	}
	return t, nil
}
