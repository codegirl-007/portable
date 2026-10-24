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

const (
	cookieName  = "portable_remote"
	cspValue    = "default-src 'self'; connect-src 'self' ws: wss:; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'"
	wsReadLimit = 1 << 20
)

// Server is the remote-access HTTP service.
type Server struct {
	Token        string
	Listen       string
	Session      string
	Dir          string
	ShellCommand string
	PublicHost   string   // sprites.app hostname; used for Origin and cookie Secure
	Exec         []string // if set, run this in a PTY instead of tmux (tests)
}

// Handler returns the HTTP handler (useful in tests).
func (s *Server) Handler() http.Handler {
	static, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/login.js", s.handleLoginJS)
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
		w.Header().Set("Content-Security-Policy", cspValue)
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
	c, err := r.Cookie(cookieName)
	return err == nil && s.validToken(c.Value)
}

func hostname(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if strings.Contains(v, "://") {
		u, err := url.Parse(v)
		if err != nil {
			return ""
		}
		v = u.Host
	}
	if host, _, err := net.SplitHostPort(v); err == nil {
		return host
	}
	return v
}

func (s *Server) cookieSecure(r *http.Request) bool {
	host := hostname(s.PublicHost)
	if host == "" {
		host = hostname(r.Host)
	}
	switch host {
	case "", "localhost", "127.0.0.1", "::1":
		return false
	default:
		return true
	}
}

func (s *Server) setAuthCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    s.Token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cookieSecure(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   30 * 24 * 3600,
	})
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
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

func (s *Server) handleLoginJS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	b, err := staticFS.ReadFile("static/login.js")
	if err != nil {
		http.Error(w, "missing login.js", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
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
<script src="/login.js"></script>
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

func (s *Server) originAllowed(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	ohost := hostname(origin)
	allowed := hostname(s.PublicHost)
	if allowed == "" {
		allowed = hostname(r.Host)
	}
	return allowed != "" && strings.EqualFold(ohost, allowed)
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
