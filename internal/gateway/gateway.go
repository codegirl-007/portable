// Package gateway is the authenticated web terminal that runs on a Sprite.
package gateway

import (
	"context"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

//go:embed static/*
var staticFS embed.FS

const cookieName = "portable_remote"

// Server is the remote-access HTTP service.
type Server struct {
	Token        string
	Listen       string
	Session      string
	Dir          string
	ShellCommand string
	Exec         []string // if set, run this in a PTY instead of tmux (tests)
}

// Handler returns the HTTP handler (useful in tests).
func (s *Server) Handler() http.Handler {
	static, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/static/", s.requireAuth(http.StripPrefix("/static/", http.FileServer(http.FS(static)))))
	mux.HandleFunc("/login", s.handleLogin)
	mux.HandleFunc("/pty", s.handlePTY)
	mux.HandleFunc("/", s.handleRoot)
	return s.headers(mux)
}

// Run listens until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	if strings.TrimSpace(s.Token) == "" {
		return fmt.Errorf("gateway: a token is required")
	}
	addr := s.Listen
	if addr == "" {
		addr = ":7681"
	}
	hs := &http.Server{
		Addr:              addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	errc := make(chan error, 1)
	go func() {
		errc <- hs.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = hs.Shutdown(shutdownCtx)
		err := <-errc
		if err == http.ErrServerClosed {
			return ctx.Err()
		}
		return err
	case err := <-errc:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func (s *Server) headers(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		if strings.HasPrefix(r.URL.Path, "/static/") || r.URL.Path == "/" {
			w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self' ws: wss:; img-src 'self' data:; style-src 'self'; script-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) validToken(got string) bool {
	want := s.Token
	if want == "" || got == "" {
		return false
	}
	if len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func (s *Server) authorized(r *http.Request) bool {
	if s.validToken(r.URL.Query().Get("t")) {
		return true
	}
	if c, err := r.Cookie(cookieName); err == nil && s.validToken(c.Value) {
		return true
	}
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") && s.validToken(strings.TrimPrefix(auth, "Bearer ")) {
		return true
	}
	return false
}

func cookieSecure(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
	}
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return false
	}
	return true
}

func (s *Server) setAuthCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    s.Token,
		Path:     "/",
		HttpOnly: true,
		Secure:   cookieSecure(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   30 * 24 * 3600,
	})
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if t := r.URL.Query().Get("t"); s.validToken(t) {
		s.setAuthCookie(w, r)
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	if !s.authorized(r) {
		s.unauth(w, http.StatusUnauthorized)
		return
	}
	b, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "missing index", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		s.unauth(w, http.StatusUnauthorized)
		return
	}
	if !s.validToken(r.Form.Get("t")) {
		s.unauth(w, http.StatusUnauthorized)
		return
	}
	s.setAuthCookie(w, r)
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) unauth(w http.ResponseWriter, status int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("WWW-Authenticate", `Bearer realm="portable"`)
	w.WriteHeader(status)
	_, _ = io.WriteString(w, `<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>portable remote</title>
<style>
body{font-family:ui-sans-serif,system-ui,sans-serif;background:#0d1117;color:#e6edf3;margin:0;padding:24px;line-height:1.4}
form{display:flex;gap:8px;flex-wrap:wrap;margin-top:16px}
input,button{font-size:16px;padding:10px;border-radius:8px;border:1px solid #30363d;background:#161b22;color:#e6edf3}
button{background:#238636;border-color:#238636}
p{color:#8b949e;max-width:36rem}
</style></head><body>
<h1>portable remote</h1>
<p>This is a private agent terminal. Open the URL printed by <code>portable remote</code>, or paste the token from that command.</p>
<form method="post" action="/login">
<input type="password" name="t" placeholder="token" autocapitalize="off" autocomplete="off" required>
<button type="submit">Open</button>
</form>
</body></html>`)
}

type resizeMsg struct {
	Type string `json:"type"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) {
			s.unauth(w, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func originAllowed(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	ohost := u.Host
	if h, _, err := net.SplitHostPort(ohost); err == nil {
		ohost = h
	}
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.EqualFold(ohost, host)
}

// LoadSessionFile reads dir and command written by `portable remote`.
func LoadSessionFile(path string) (dir, command string, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	var v struct {
		Dir     string `json:"dir"`
		Command string `json:"command"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return "", "", err
	}
	return v.Dir, v.Command, nil
}

func homePath(rel string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "/home/sprite"
	}
	return home + rel
}

// DefaultTokenFile is ~/.config/portable/remote.token on the Sprite.
func DefaultTokenFile() string {
	return homePath("/.config/portable/remote.token")
}

// DefaultSessionFile is ~/.config/portable/remote-session.json on the Sprite.
func DefaultSessionFile() string {
	return homePath("/.config/portable/remote-session.json")
}
