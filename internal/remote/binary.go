package remote

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var (
	releaseDownloadBase = "https://github.com/codegirl-007/portable/releases/download"
	httpClient          = &http.Client{Timeout: 60 * time.Second}
)

// LinuxBinary places a linux portable executable at destPath (basename "portable").
// cleanup removes any temp dir; destPath is inside that dir unless it is the
// running executable itself.
func LinuxBinary(goarch, version, moduleRoot string) (destPath string, cleanup func(), err error) {
	nop := func() {}
	if runtime.GOOS == "linux" && runtime.GOARCH == goarch {
		exe, err := os.Executable()
		if err != nil {
			return "", nop, err
		}
		return exe, nop, nil
	}

	dir, err := os.MkdirTemp("", "portable-linux-*")
	if err != nil {
		return "", nop, err
	}
	cleanup = func() { _ = os.RemoveAll(dir) }
	dest := filepath.Join(dir, "portable")

	if moduleRoot != "" {
		if err := crossCompile(moduleRoot, goarch, dest); err != nil {
			cleanup()
			return "", nop, err
		}
		return dest, cleanup, nil
	}
	if version != "" && version != "dev" && version != "(devel)" {
		if err := downloadRelease(version, goarch, dest); err != nil {
			cleanup()
			return "", nop, err
		}
		return dest, cleanup, nil
	}
	cleanup()
	return "", nop, fmt.Errorf("cannot build a linux/%s gateway from this %s/%s %q binary; run portable from a git checkout of github.com/codegirl-007/portable, or install a GitHub release (go install builds cannot copy a linux binary)",
		goarch, runtime.GOOS, runtime.GOARCH, version)
}

func crossCompile(moduleRoot, goarch, dest string) error {
	cmd := exec.Command("go", "build", "-o", dest, ".")
	cmd.Dir = moduleRoot
	cmd.Env = append(os.Environ(),
		"GOOS=linux",
		"GOARCH="+goarch,
		"CGO_ENABLED=0",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go build linux/%s: %w: %s", goarch, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func downloadRelease(version, goarch, dest string) error {
	ver := strings.TrimPrefix(version, "v")
	tag := "v" + ver
	name := fmt.Sprintf("portable_%s_linux_%s.tar.gz", ver, goarch)
	base := strings.TrimRight(releaseDownloadBase, "/") + "/" + tag + "/"

	sums, err := httpGet(base + "checksums.txt")
	if err != nil {
		return fmt.Errorf("download checksums.txt: %w", err)
	}
	want, err := checksumFor(string(sums), name)
	if err != nil {
		return err
	}
	archive, err := httpGet(base + name)
	if err != nil {
		return fmt.Errorf("download %s: %w", name, err)
	}
	sum := sha256.Sum256(archive)
	got := hex.EncodeToString(sum[:])
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("checksum mismatch for %s: got %s, want %s", name, got, want)
	}
	return extractPortable(bytes.NewReader(archive), dest, name)
}

func httpGet(rawURL string) ([]byte, error) {
	resp, err := httpClient.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: HTTP %d", rawURL, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func checksumFor(sums, filename string) (string, error) {
	for _, line := range strings.Split(sums, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		file := strings.TrimPrefix(fields[len(fields)-1], "*")
		if filepath.Base(file) == filename {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("checksums.txt has no entry for %s", filename)
}

func extractPortable(r io.Reader, dest, name string) error {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		base := filepath.Base(hdr.Name)
		if hdr.Typeflag != tar.TypeReg || (base != "portable" && base != "portable.exe") {
			continue
		}
		f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(f, tr)
		closeErr := f.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	}
	return fmt.Errorf("archive %s did not contain portable", name)
}

// FindModuleRoot walks parents of start looking for this module's go.mod.
func FindModuleRoot(start string) string {
	dir := start
	for {
		b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil && strings.HasPrefix(string(b), "module github.com/codegirl-007/portable") {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
