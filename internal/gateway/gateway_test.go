package gateway

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

const testToken = "tokentokentokentokentokentoken12"

func testServer(t *testing.T) *httptest.Server {
	t.Helper()
	s := &Server{Token: testToken, Exec: []string{"sh", "-c", "printf hi-from-pty; exec cat"}}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return ts
}

func loginClient(t *testing.T, ts *httptest.Server) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	c := &http.Client{Jar: jar}
	res, err := c.Post(ts.URL+"/login", "application/x-www-form-urlencoded", strings.NewReader("t="+testToken))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d", res.StatusCode)
	}
	return c
}

func TestRejectsMissingAuth(t *testing.T) {
	ts := testServer(t)
	res, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	if bytes.Contains(body, []byte("id=\"term\"")) {
		t.Fatal("unauthenticated response must not include the terminal")
	}
	if !bytes.Contains(body, []byte("/login.js")) {
		t.Fatal("401 page should load login.js for fragment tokens")
	}
}

func TestQueryTokenDoesNotAuth(t *testing.T) {
	ts := testServer(t)
	res, err := http.Get(ts.URL + "/?t=" + testToken)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d", res.StatusCode)
	}
}

func TestLoginSetsCookieAndServesTerminal(t *testing.T) {
	ts := testServer(t)
	c := loginClient(t, ts)
	res, err := c.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	if !bytes.Contains(body, []byte("id=\"term\"")) {
		t.Fatalf("expected terminal html, got %s", body[:min(200, len(body))])
	}
	if !bytes.Contains(body, []byte("data-key=\"esc\"")) || !bytes.Contains(body, []byte("data-key=\"c-c\"")) {
		t.Fatal("expected phone special-key buttons")
	}
	if bytes.Contains(body, []byte("user-scalable=no")) {
		t.Fatal("viewport should not disable scaling")
	}
	static, err := c.Get(ts.URL + "/static/app.js")
	if err != nil {
		t.Fatal(err)
	}
	defer static.Body.Close()
	if static.StatusCode != http.StatusOK {
		t.Fatalf("static status = %d", static.StatusCode)
	}
	js, _ := io.ReadAll(static.Body)
	if !bytes.Contains(js, []byte("maxFailures")) || !bytes.Contains(js, []byte("status === 401")) {
		t.Fatal("app.js should back off and reload on 401")
	}
}

func TestLoginJSIsPublic(t *testing.T) {
	ts := testServer(t)
	res, err := http.Get(ts.URL + "/login.js")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	if !bytes.Contains(body, []byte("#t=")) || !bytes.Contains(body, []byte("/login")) {
		t.Fatalf("login.js should POST the fragment token, got %s", body)
	}
}

func TestCSPAllowsInlineStyleNotInlineScript(t *testing.T) {
	ts := testServer(t)
	res, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	csp := res.Header.Get("Content-Security-Policy")
	if !strings.Contains(csp, "style-src 'self' 'unsafe-inline'") {
		t.Fatalf("csp = %q", csp)
	}
	if strings.Contains(csp, "script-src 'self' 'unsafe-inline'") || !strings.Contains(csp, "script-src 'self'") {
		t.Fatalf("script-src should stay 'self': %q", csp)
	}
}

func TestStaticRequiresAuth(t *testing.T) {
	ts := testServer(t)
	res, err := http.Get(ts.URL + "/static/app.js")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d", res.StatusCode)
	}
}

func TestLoginRejectsBadToken(t *testing.T) {
	ts := testServer(t)
	res, err := http.Post(ts.URL+"/login", "application/x-www-form-urlencoded", strings.NewReader("t=nope"))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d", res.StatusCode)
	}
}

func TestPTYRequiresAuth(t *testing.T) {
	ts := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	u := "ws" + strings.TrimPrefix(ts.URL, "http") + "/pty"
	_, _, err := websocket.Dial(ctx, u, nil)
	if err == nil {
		t.Fatal("expected websocket auth failure")
	}
}

func dialPTY(t *testing.T, ctx context.Context, ts *httptest.Server, c *http.Client) *websocket.Conn {
	t.Helper()
	u := "ws" + strings.TrimPrefix(ts.URL, "http") + "/pty"
	conn, _, err := websocket.Dial(ctx, u, &websocket.DialOptions{HTTPClient: c})
	if err != nil {
		t.Fatal(err)
	}
	return conn
}

func TestPTYEcho(t *testing.T) {
	ts := testServer(t)
	c := loginClient(t, ts)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn := dialPTY(t, ctx, ts, c)
	defer conn.Close(websocket.StatusNormalClosure, "")

	deadline := time.Now().Add(3 * time.Second)
	var got []byte
	for time.Now().Before(deadline) {
		_, data, err := conn.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, data...)
		if bytes.Contains(got, []byte("hi-from-pty")) {
			return
		}
	}
	t.Fatalf("pty output %q missing hi-from-pty", got)
}

func TestPTYDisconnectKillsProcess(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "pid")
	s := &Server{Token: testToken, Exec: []string{"sh", "-c", "echo $$ >" + pidFile + "; exec sleep 60"}}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	c := loginClient(t, ts)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	conn := dialPTY(t, ctx, ts, c)

	var pid int
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		b, err := os.ReadFile(pidFile)
		if err == nil {
			pid, err = strconv.Atoi(strings.TrimSpace(string(b)))
			if err == nil && pid > 0 {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if pid == 0 {
		t.Fatal("pty child did not write pid")
	}
	if err := conn.Close(websocket.StatusNormalClosure, ""); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if err := exec.Command("kill", "-0", strconv.Itoa(pid)).Run(); err != nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("process %d still running after websocket close", pid)
}

func TestValidTokenRejectsEmptyAndWrongLength(t *testing.T) {
	s := &Server{Token: "abcdef"}
	if s.validToken("") || s.validToken("abc") || s.validToken("ABCDEF") {
		t.Fatal("expected mismatch")
	}
	if !s.validToken("abcdef") {
		t.Fatal("expected match")
	}
}

func TestOriginAllowed(t *testing.T) {
	s := &Server{}
	r := httptest.NewRequest(http.MethodGet, "http://example.sprites.app/pty", nil)
	r.Host = "example.sprites.app"
	r.Header.Set("Origin", "https://evil.example")
	if s.originAllowed(r) {
		t.Fatal("cross-origin should be rejected")
	}
	r.Header.Set("Origin", "https://example.sprites.app")
	if !s.originAllowed(r) {
		t.Fatal("same origin should be allowed")
	}
	r.Header.Del("Origin")
	if !s.originAllowed(r) {
		t.Fatal("missing origin should be allowed")
	}
}

func TestOriginAllowedPublicHost(t *testing.T) {
	s := &Server{PublicHost: "demo-abc.sprites.app"}
	r := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:7681/pty", nil)
	r.Host = "127.0.0.1:7681"
	r.Header.Set("Origin", "https://demo-abc.sprites.app")
	if !s.originAllowed(r) {
		t.Fatal("public host should win over rewritten Host")
	}
	r.Header.Set("Origin", "https://evil.example")
	if s.originAllowed(r) {
		t.Fatal("evil origin should be rejected even if Host is loopback")
	}
}

func TestCookieSecureUsesPublicHost(t *testing.T) {
	s := &Server{PublicHost: "demo-abc.sprites.app"}
	r := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:7681/", nil)
	r.Host = "127.0.0.1:7681"
	if !s.cookieSecure(r) {
		t.Fatal("cookie should be Secure when PublicHost is sprites.app")
	}
	s.PublicHost = ""
	if s.cookieSecure(r) {
		t.Fatal("loopback Host without PublicHost should not set Secure")
	}
}

func TestRunRequiresToken(t *testing.T) {
	err := (&Server{}).Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "token") {
		t.Fatalf("err = %v", err)
	}
}

func TestPTYTmuxSession(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not available")
	}
	session := fmt.Sprintf("portable-test-%d", os.Getpid())
	t.Cleanup(func() {
		_ = exec.Command("tmux", "kill-session", "-t", session).Run()
	})
	s := &Server{
		Token:        testToken,
		Session:      session,
		Dir:          t.TempDir(),
		ShellCommand: "printf tmux-gateway-ok; exec cat",
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	c := loginClient(t, ts)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	conn := dialPTY(t, ctx, ts, c)
	defer conn.Close(websocket.StatusNormalClosure, "")

	deadline := time.Now().Add(5 * time.Second)
	var got []byte
	for time.Now().Before(deadline) {
		_, data, err := conn.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, data...)
		if bytes.Contains(got, []byte("tmux-gateway-ok")) {
			return
		}
	}
	t.Fatalf("tmux pty output %q missing tmux-gateway-ok", got)
}
