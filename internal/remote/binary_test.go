package remote

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestChecksumFor(t *testing.T) {
	sums := "abc123  portable_1.0.0_linux_amd64.tar.gz\n"
	got, err := checksumFor(sums, "portable_1.0.0_linux_amd64.tar.gz")
	if err != nil || got != "abc123" {
		t.Fatalf("%q %v", got, err)
	}
	if _, err := checksumFor(sums, "missing.tar.gz"); err == nil {
		t.Fatal("expected missing")
	}
}

func TestLinuxBinaryDevWithoutSource(t *testing.T) {
	if runtime.GOOS == "linux" && runtime.GOARCH == "riscv64" {
		t.Skip("matching arch returns the current binary")
	}
	_, _, err := LinuxBinary("riscv64", "dev", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "git checkout") || !strings.Contains(err.Error(), "go install") {
		t.Fatalf("err = %v", err)
	}
}

func TestDownloadReleaseVerifiesChecksum(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	body := []byte("#!/bin/sh\necho portable\n")
	if err := tw.WriteHeader(&tar.Header{Name: "portable", Mode: 0755, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	archive := buf.Bytes()
	sum := sha256.Sum256(archive)
	name := "portable_1.2.3_linux_amd64.tar.gz"

	mux := http.NewServeMux()
	mux.HandleFunc("/v1.2.3/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), name)
	})
	mux.HandleFunc("/v1.2.3/"+name, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive)
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	oldBase, oldClient := releaseDownloadBase, httpClient
	releaseDownloadBase = ts.URL
	httpClient = ts.Client()
	t.Cleanup(func() {
		releaseDownloadBase = oldBase
		httpClient = oldClient
	})

	dest := filepath.Join(t.TempDir(), "portable")
	if err := downloadRelease("1.2.3", "amd64", dest); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("extracted %q", got)
	}
}

func TestDownloadReleaseChecksumMismatch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1.2.3/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "deadbeef  portable_1.2.3_linux_amd64.tar.gz")
	})
	mux.HandleFunc("/v1.2.3/portable_1.2.3_linux_amd64.tar.gz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("nope"))
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	oldBase, oldClient := releaseDownloadBase, httpClient
	releaseDownloadBase = ts.URL
	httpClient = ts.Client()
	t.Cleanup(func() {
		releaseDownloadBase = oldBase
		httpClient = oldClient
	})
	err := downloadRelease("1.2.3", "amd64", filepath.Join(t.TempDir(), "portable"))
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("err = %v", err)
	}
}
