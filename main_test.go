package main

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestLoadConfigDefaultsAndUnicode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`alist:
  username: "admin"
tasks:
  - name: "鏂囨。"
    local: "D:/My Documents"
    remote: "/百度网盘备份/Documents"
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AList.URL != "http://127.0.0.1:5244" {
		t.Fatalf("unexpected default url: %s", cfg.AList.URL)
	}
	if cfg.AList.PasswordEnv != "ALIST_PASSWORD" {
		t.Fatalf("unexpected password env: %s", cfg.AList.PasswordEnv)
	}
	if cfg.Rclone.Remote != "alist_baidu" {
		t.Fatalf("unexpected remote: %s", cfg.Rclone.Remote)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
}

func TestValidateReportsMissingFields(t *testing.T) {
	err := (Config{}).Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "alist.url") || !strings.Contains(err.Error(), "tasks") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSelectTasks(t *testing.T) {
	tasks := []Task{{Name: "a"}, {Name: "b"}}
	all, err := SelectTasks(tasks, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("expected all tasks, got %d", len(all))
	}
	one, err := SelectTasks(tasks, "b", false)
	if err != nil {
		t.Fatal(err)
	}
	if one[0].Name != "b" {
		t.Fatalf("unexpected task: %v", one)
	}
	if _, err := SelectTasks(tasks, "", false); err == nil {
		t.Fatal("expected error when multiple tasks and no selector")
	}
}

func TestBuildRcloneArgs(t *testing.T) {
	cfg := Config{
		Rclone: RcloneConfig{
			Remote:     "alist_baidu",
			ConfigFile: ".alist-sync/rclone.conf",
			Transfers:  4,
			Checkers:   8,
		},
	}
	task := Task{Name: "documents", Local: "D:/My Documents", Remote: "/鐧惧害缃戠洏澶囦唤/Documents"}

	got := BuildRcloneArgs("dry-run", cfg, task)
	want := []string{
		"sync",
		toNativePath("D:/My Documents"),
		"alist_baidu:鐧惧害缃戠洏澶囦唤/Documents",
		"--config", toNativePath(".alist-sync/rclone.conf"),
		"--transfers", "4",
		"--checkers", "8",
		"--progress",
		"--dry-run", "--combined", "-",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dry-run args mismatch\n got: %#v\nwant: %#v", got, want)
	}

	update := BuildRcloneArgs("update", cfg, task)
	if update[0] != "copy" {
		t.Fatalf("update should use copy, got %s", update[0])
	}
	sync := BuildRcloneArgs("sync", cfg, task)
	if sync[0] != "sync" {
		t.Fatalf("sync should use sync, got %s", sync[0])
	}
}

func TestSyncWithoutYesPreviewsThenStops(t *testing.T) {
	dir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWD)
	if err := os.MkdirAll(filepath.Join(".alist-sync", "tools"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(".alist-sync", "tools", exeName("rclone")), []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "config.yaml")
	err = os.WriteFile(cfgPath, []byte(`alist:
  url: "http://127.0.0.1:5244"
  username: "admin"
rclone:
  remote: "alist_baidu"
  config_file: ".alist-sync/rclone.conf"
tasks:
  - name: "documents"
    local: "D:/Documents"
    remote: "/backup"
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	var calls [][]string
	r := Runner{
		stdout: os.Stdout,
		stderr: os.Stderr,
		exec: func(name string, args ...string) error {
			calls = append(calls, append([]string{name}, args...))
			return nil
		},
	}
	err = r.cmdTransfer("sync", []string{"--config", cfgPath, "documents"})
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("expected --yes error, got %v", err)
	}
	if len(calls) != 1 || calls[0][1] != "sync" || !containsArg(calls[0], "--dry-run") {
		t.Fatalf("expected one dry-run sync preview, got %#v", calls)
	}
}

func containsArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func TestToNativePath(t *testing.T) {
	got := toNativePath(".alist-sync/rclone.conf")
	if runtime.GOOS == "windows" && strings.Contains(got, "/") {
		t.Fatalf("expected native windows separators, got %s", got)
	}
}
