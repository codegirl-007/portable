package gateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/coder/websocket"
	"github.com/creack/pty"
)

func (s *Server) handlePTY(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		s.unauth(w, http.StatusUnauthorized)
		return
	}
	if !originAllowed(r) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	cmd, err := s.ptyCommand()
	if err != nil {
		_ = conn.Close(websocket.StatusInternalError, "pty")
		return
	}
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor")
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 80})
	if err != nil {
		_ = conn.Close(websocket.StatusInternalError, "pty start")
		return
	}
	defer func() {
		_ = ptmx.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	ctx := r.Context()
	go s.wsToPTY(ctx, conn, ptmx)
	s.ptyToWS(ctx, conn, ptmx)
}

func (s *Server) ptyCommand() (*exec.Cmd, error) {
	if len(s.Exec) > 0 {
		return exec.Command(s.Exec[0], s.Exec[1:]...), nil
	}
	session := s.Session
	if session == "" {
		session = "portable"
	}
	args := []string{"-u", "new-session", "-A", "-s", session}
	if s.Dir != "" {
		args = append(args, "-c", s.Dir)
	}
	if strings.TrimSpace(s.ShellCommand) != "" {
		args = append(args, "--", "sh", "-lc", s.ShellCommand)
	}
	return exec.Command("tmux", args...), nil
}

func (s *Server) wsToPTY(ctx context.Context, conn *websocket.Conn, ptmx *os.File) {
	for {
		typ, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		if typ == websocket.MessageText {
			var msg resizeMsg
			if json.Unmarshal(data, &msg) == nil && msg.Type == "resize" && msg.Cols > 0 && msg.Rows > 0 {
				_ = pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(msg.Rows), Cols: uint16(msg.Cols)})
			}
			continue
		}
		if _, err := ptmx.Write(data); err != nil {
			return
		}
	}
}

func (s *Server) ptyToWS(ctx context.Context, conn *websocket.Conn, ptmx *os.File) {
	buf := make([]byte, 8192)
	for {
		n, err := ptmx.Read(buf)
		if n > 0 {
			if werr := conn.Write(ctx, websocket.MessageBinary, buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			if err != io.EOF {
				_ = conn.Close(websocket.StatusNormalClosure, "")
			}
			return
		}
	}
}
