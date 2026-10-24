package remote

import (
	"strings"
)

func shq(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// TmuxAttachOrCreate returns a shell snippet that installs tmux if needed,
// then attaches to the named session or creates it running innerCommand.
// If tmux cannot be installed, it falls back to running innerCommand directly
// so `portable agent` still works on an unprovisioned workspace.
func TmuxAttachOrCreate(session, dir, innerCommand string) string {
	if session == "" {
		session = SessionName
	}
	var b strings.Builder
	b.WriteString(`export PATH="$HOME/.local/bin:$PATH"; `)
	b.WriteString(`if ! command -v tmux >/dev/null 2>&1; then `)
	b.WriteString(`sudo apt-get update -qq >/dev/null 2>&1 && sudo apt-get install -y -qq tmux >/dev/null 2>&1 || true; `)
	b.WriteString(`fi; `)
	b.WriteString(`export TERM="${TERM:-xterm-256color}"; `)
	b.WriteString(`if ! command -v tmux >/dev/null 2>&1; then `)
	b.WriteString(`exec sh -c ` + shq(innerCommand) + `; `)
	b.WriteString(`fi; `)
	b.WriteString(`if tmux has-session -t ` + shq(session) + ` 2>/dev/null; then `)
	b.WriteString(`exec tmux attach-session -t ` + shq(session) + `; `)
	b.WriteString(`fi; `)
	b.WriteString(`exec tmux -u new-session -s ` + shq(session))
	if strings.TrimSpace(dir) != "" {
		b.WriteString(` -c ` + shq(dir))
	}
	b.WriteString(` -- sh -lc ` + shq(innerCommand))
	return b.String()
}
