package remote

import (
	"archive/tar"
	"compress/gzip"
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
		if err := crossCompile(moduleRoot, goarch, dest); err == nil {
			return dest, cleanup, nil
		}
	}
	if version != "" && version != "dev" {
		if err := downloadRelease(version, goarch, dest); err == nil {
			return dest, cleanup, nil
		}
	}
	if moduleRoot == "" {
		cleanup()
		return "", nop, fmt.Errorf("need Go module source or a GitHub release to install the linux gateway binary")
	}
	if err := crossCompile(moduleRoot, goarch, dest); err != nil {
		cleanup()
		return "", nop, err
	}
	return dest, cleanup, nil
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
	url := fmt.Sprintf("https://github.com/codegirl-007/portable/releases/download/%s/%s", tag, name)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}
	gr, err := gzip.NewReader(resp.Body)
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
