// Package remote implements phone/browser access to a Sprite agent session.
package remote

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
)

const (
	// ServiceName is the sprite-env service that owns the Sprite HTTP URL.
	ServiceName = "portable-remote"
	// SessionName is the tmux session shared by `portable agent` and the web UI.
	SessionName = "portable"
	// ListenAddr is the gateway bind address (Sprite HTTP port).
	ListenAddr = ":7681"
	// HTTPPort is routed from the Sprite URL.
	HTTPPort = 7681
	// BinaryPath is where the linux portable binary is installed on the Sprite.
	BinaryPath = "/home/sprite/.local/bin/portable"
	// TokenRel is the home-relative token path (Sprite home, not the synced project).
	TokenRel = ".config/portable/remote.token"
	// SessionRel is the home-relative session JSON path.
	SessionRel = ".config/portable/remote-session.json"
	QueryToken = "t"
)

// NewToken returns a 256-bit hex token for remote HTTP auth.
func NewToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// TokenFile is the Sprite path of the remote auth token.
func TokenFile() string { return "/home/sprite/" + TokenRel }

// SessionFile is the Sprite path of the tmux session command JSON.
func SessionFile() string { return "/home/sprite/" + SessionRel }

// AccessURL appends the auth token to the Sprite HTTPS URL.
func AccessURL(base, token string) (string, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		return "", fmt.Errorf("sprite URL is empty")
	}
	if token == "" {
		return "", fmt.Errorf("remote token is empty")
	}
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" {
		u.Scheme = "https"
	}
	q := u.Query()
	q.Set(QueryToken, token)
	u.RawQuery = q.Encode()
	u.Fragment = ""
	return u.String(), nil
}

var spritesURL = regexp.MustCompile(`https://[a-zA-Z0-9._-]+\.sprites\.app`)

// SpriteHTTP is the Sprite's public HTTPS URL and URL-auth mode.
type SpriteHTTP struct {
	URL  string
	Auth string
}

// ParseSpriteHTTP extracts a sprites.app URL and auth mode from CLI output.
func ParseSpriteHTTP(out string) SpriteHTTP {
	var info SpriteHTTP
	var api struct {
		URL         string `json:"url"`
		URLSettings struct {
			Auth string `json:"auth"`
		} `json:"url_settings"`
	}
	trim := strings.TrimSpace(out)
	if json.Unmarshal([]byte(trim), &api) == nil && api.URL != "" {
		info.URL = strings.TrimRight(api.URL, "/")
		info.Auth = strings.TrimSpace(api.URLSettings.Auth)
	}
	if info.URL == "" {
		info.URL = strings.TrimRight(spritesURL.FindString(out), "/")
	}
	low := strings.ToLower(out)
	if info.Auth == "" {
		switch {
		case strings.Contains(low, "public"):
			info.Auth = "public"
		case strings.Contains(low, "sprite"):
			info.Auth = "sprite"
		}
	}
	return info
}

// LinuxGOARCH maps uname -m to a Go GOARCH value.
func LinuxGOARCH(unameM string) (string, error) {
	switch strings.TrimSpace(unameM) {
	case "x86_64", "amd64":
		return "amd64", nil
	case "aarch64", "arm64":
		return "arm64", nil
	default:
		return "", fmt.Errorf("unsupported sprite architecture %q", strings.TrimSpace(unameM))
	}
}

// SessionJSON is the file the gateway reads for a new tmux session.
type SessionJSON struct {
	Dir     string `json:"dir"`
	Command string `json:"command"`
}

// EncodeSession marshals session JSON.
func EncodeSession(dir, command string) ([]byte, error) {
	return json.Marshal(SessionJSON{Dir: dir, Command: command})
}

// BinaryDest is the remote path of the portable binary.
func BinaryDest() string { return BinaryPath }

// RelSlash joins home-relative path segments with slashes.
func RelSlash(parts ...string) string { return path.Join(parts...) }
