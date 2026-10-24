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
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func testServer(t *testing.T) *httptest.Server {
	t.Helper()
	s := &Server{Token: "tokentokentokentokentokentoken12", Exec: []string{"sh", "-c", "printf hi-from-pty; exec cat"}}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return ts
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
}

func TestRejectsBadToken(t *testing.T) {
	ts := testServer(t)
	res, err := http.Get(ts.URL + "/?t=nope")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d", res.StatusCode)
	}
}

func TestTokenQuerySetsCookieAndServesTerminal(t *testing.T) {
	ts := testServer(t)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	c := &http.Client{Jar: jar}
	res, err := c.Get(ts.URL + "/?t=tokentokentokentokentokentoken12")
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
	static, err := c.Get(ts.URL + "/static/app.js")
	if err != nil {
		t.Fatal(err)
	}
	defer static.Body.Close()
	if static.StatusCode != http.StatusOK {
		t.Fatalf("static status = %d", static.StatusCode)
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

func TestBearerAuth(t *testing.T) {
	ts := testServer(t)
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/", nil)
	req.Header.Set("Authorization", "Bearer tokentokentokentokentokentoken12")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
}

func TestLoginForm(t *testing.T) {
	ts := testServer(t)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	c := &http.Client{Jar: jar}
	res, err := c.Post(ts.URL+"/login", "application/x-www-form-urlencoded", strings.NewReader("t=tokentokentokentokentokentoken12"))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
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

func TestPTYEcho(t *testing.T) {
	ts := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	u := "ws" + strings.TrimPrefix(ts.URL, "http") + "/pty?t=tokentokentokentokentokentoken12"
	conn, _, err := websocket.Dial(ctx, u, nil)
	if err != nil {
		t.Fatal(err)
	}
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
	r := httptest.NewRequest(http.MethodGet, "http://example.sprites.app/pty", nil)
	r.Host = "example.sprites.app"
	r.Header.Set("Origin", "https://evil.example")
	if originAllowed(r) {
		t.Fatal("cross-origin should be rejected")
	}
	r.Header.Set("Origin", "https://example.sprites.app")
	if !originAllowed(r) {
		t.Fatal("same origin should be allowed")
	}
	r.Header.Del("Origin")
	if !originAllowed(r) {
		t.Fatal("missing origin should be allowed")
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
		Token:        "tokentokentokentokentokentoken12",
		Session:      session,
		Dir:          t.TempDir(),
		ShellCommand: "printf tmux-gateway-ok; exec cat",
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	u := "ws" + strings.TrimPrefix(ts.URL, "http") + "/pty?t=tokentokentokentokentokentoken12"
	conn, _, err := websocket.Dial(ctx, u, nil)
	if err != nil {
		t.Fatal(err)
	}
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
