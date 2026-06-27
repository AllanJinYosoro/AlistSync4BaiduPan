package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
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

func ensureTool(name string, force bool, w io.Writer) error {
	if !force {
		if p, err := exec.LookPath(exeName(name)); err == nil {
			fmt.Fprintf(w, "%s found in PATH: %s\n", name, p)
			return nil
		}
		if p := localToolPath(name); fileExists(p) {
			fmt.Fprintf(w, "%s found locally: %s\n", name, p)
			return nil
		}
	}
	fmt.Fprintf(w, "downloading %s...\n", name)
	switch name {
	case "rclone":
		return downloadRclone(localToolPath(name))
	case "alist":
		return downloadAList(localToolPath(name))
	default:
		return fmt.Errorf("unknown tool %q", name)
	}
}

func findTool(name string) (string, error) {
	if p, err := exec.LookPath(exeName(name)); err == nil {
		return p, nil
	}
	if p := localToolPath(name); fileExists(p) {
		return p, nil
	}
	return "", fmt.Errorf("%s not found; run `bdp-sync setup deps` first", name)
}

func localToolPath(name string) string {
	return filepath.Join(toNativePath(toolsDir), exeName(name))
}

func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func downloadRclone(dest string) error {
	goos := runtime.GOOS
	if goos == "darwin" {
		goos = "osx"
	}
	arch := runtime.GOARCH
	url := fmt.Sprintf("https://downloads.rclone.org/rclone-current-%s-%s.zip", goos, arch)
	data, err := download(url)
	if err != nil {
		return err
	}
	return extractExecutableFromZip(data, exeName("rclone"), dest)
}

func downloadAList(dest string) error {
	type asset struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	}
	type release struct {
		Assets []asset `json:"assets"`
	}
	data, err := download("https://api.github.com/repos/AlistGo/alist/releases/latest")
	if err != nil {
		return err
	}
	var rel release
	if err := json.Unmarshal(data, &rel); err != nil {
		return err
	}
	chosen := ""
	for _, a := range rel.Assets {
		n := strings.ToLower(a.Name)
		if strings.Contains(n, runtime.GOOS) && strings.Contains(n, runtime.GOARCH) && (strings.HasSuffix(n, ".zip") || strings.HasSuffix(n, ".tar.gz")) {
			chosen = a.URL
			break
		}
	}
	if chosen == "" {
		return fmt.Errorf("could not find AList release asset for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	archive, err := download(chosen)
	if err != nil {
		return err
	}
	if strings.HasSuffix(strings.ToLower(chosen), ".zip") {
		return extractExecutableFromZip(archive, exeName("alist"), dest)
	}
	return extractExecutableFromTarGz(archive, exeName("alist"), dest)
}

func download(rawurl string) ([]byte, error) {
	client := http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(rawurl)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download %s failed: %s", rawurl, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func extractExecutableFromZip(data []byte, executable string, dest string) error {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	for _, f := range reader.File {
		if filepath.Base(f.Name) != executable {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		return writeExecutable(rc, dest)
	}
	return fmt.Errorf("%s not found in zip archive", executable)
}

func extractExecutableFromTarGz(data []byte, executable string, dest string) error {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if filepath.Base(h.Name) == executable {
			return writeExecutable(tr, dest)
		}
	}
	return fmt.Errorf("%s not found in tar.gz archive", executable)
}

func writeExecutable(r io.Reader, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, r)
	return err
}

func runOutput(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
