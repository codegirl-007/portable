package remote

import (
	"fmt"
	"strconv"
	"strings"
)

// EnableRequest is the Sprite-side install for remote access.
type EnableRequest struct {
	Token   string
	Dir     string
	Command string
}

// EnableScript installs tmux, writes the token/session files, and creates
// the HTTP service that runs `portable gateway`.
func EnableScript(req EnableRequest) (string, error) {
	if strings.TrimSpace(req.Token) == "" {
		return "", fmt.Errorf("remote: token is required")
	}
	sess, err := EncodeSession(req.Dir, req.Command)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	w := func(format string, args ...any) { b.WriteString(fmt.Sprintf(format, args...) + "\n") }

	w("#!/usr/bin/env bash")
	w("set -euo pipefail")
	w(`export PATH="$HOME/.local/bin:$PATH"`)
	w(`log() { printf '\n==> %%s\n' "$*"; }`)
	w("")
	w(`log "tmux"`)
	w(`if ! command -v tmux >/dev/null 2>&1; then sudo apt-get update -qq && sudo apt-get install -y -qq tmux; fi`)
	w(`command -v tmux >/dev/null 2>&1 || { echo "tmux is not installed" >&2; exit 1; }`)
	w("")
	w(`BIN=%s`, shq(BinaryPath))
	w(`if [ ! -x "$BIN" ]; then echo "missing $BIN" >&2; exit 1; fi`)
	w("")
	w(`mkdir -p "$HOME/.config/portable" "$HOME/.local/bin"`)
	w(`chmod 700 "$HOME/.config/portable"`)
	w(`umask 077`)
	w(`cat > "$HOME/%s" <<'PORTABLE_REMOTE_TOKEN'`, TokenRel)
	w("%s", strings.TrimRight(req.Token, "\n"))
	w(`PORTABLE_REMOTE_TOKEN`)
	w(`chmod 600 "$HOME/%s"`, TokenRel)
	w(`cat > "$HOME/%s" <<'PORTABLE_REMOTE_SESSION'`, SessionRel)
	w("%s", strings.TrimRight(string(sess), "\n"))
	w(`PORTABLE_REMOTE_SESSION`)
	w(`chmod 600 "$HOME/%s"`, SessionRel)
	w("")
	w(`log "remote service"`)
	w(`sprite-env services delete %s >/dev/null 2>&1 || true`, ServiceName)
	w(`sprite-env services create %s --cmd %s --args %s --http-port %s --no-stream`,
		ServiceName, shq(BinaryPath), shq("gateway,--listen,"+ListenAddr), strconv.Itoa(HTTPPort))
	w(`log "remote ready"`)
	return b.String(), nil
}

// DisableScript removes the remote HTTP service (URL auth is flipped from the laptop).
func DisableScript() string {
	return fmt.Sprintf("#!/usr/bin/env bash\nset -euo pipefail\nsprite-env services delete %s >/dev/null 2>&1 || true\n", ServiceName)
}

// ReadTokenScript prints the token file if it exists.
func ReadTokenScript() string {
	return fmt.Sprintf("test -f %s && cat %s || true\n", shq(TokenFile()), shq(TokenFile()))
}
