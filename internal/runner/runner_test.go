package runner

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"bdp-sync/internal/alist"
	"bdp-sync/internal/config"
	"bdp-sync/internal/deps"
	"bdp-sync/internal/filename"
	"bdp-sync/internal/rclone"
)

func TestLoadConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`alist:
  username: "admin"
tasks:
  - name: "documents"
    local: "D:/My Documents"
    remote: "/BaiduPanBackup/Documents"
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AList.URL != "http://127.0.0.1:5244" {
		t.Fatalf("unexpected default url: %s", cfg.AList.URL)
	}
	if cfg.AList.PasswordEnv != "ALIST_PASSWORD" {
		t.Fatalf("unexpected password env: %s", cfg.AList.PasswordEnv)
	}
	if cfg.AList.StartupTimeoutSeconds != 30 {
		t.Fatalf("unexpected startup timeout: %d", cfg.AList.StartupTimeoutSeconds)
	}
	if cfg.Rclone.Remote != "alist_baidu" {
		t.Fatalf("unexpected remote: %s", cfg.Rclone.Remote)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
}

func TestLoadConfigTaskExcludes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`alist:
  username: "admin"
tasks:
  - name: "documents"
    local: "D:/My Documents"
    remote: "/BaiduPanBackup/Documents"
    excludes:
      - "private/**"
      - "tmp/**"
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"private/**", "tmp/**"}
	if !reflect.DeepEqual(cfg.Tasks[0].Excludes, want) {
		t.Fatalf("task excludes mismatch\n got: %#v\nwant: %#v", cfg.Tasks[0].Excludes, want)
	}
}

func TestValidateReportsMissingFields(t *testing.T) {
	err := (config.Config{}).Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "alist.url") || !strings.Contains(err.Error(), "tasks") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDotEnvDoesNotOverrideExistingEnv(t *testing.T) {
	t.Setenv("ALIST_PASSWORD", "from-env")
	err := config.ParseDotEnv(strings.NewReader(`ALIST_PASSWORD=from-file
NEW_VALUE="from-dot-env"
`))
	if err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("ALIST_PASSWORD"); got != "from-env" {
		t.Fatalf("expected existing env to win, got %q", got)
	}
	if got := os.Getenv("NEW_VALUE"); got != "from-dot-env" {
		t.Fatalf("expected .env value, got %q", got)
	}
}

func TestSaveRawConfigDoesNotOverwriteInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	original := []byte(`alist:
  username: "admin"
tasks:
  - name: "documents"
    local: "D:/Docs"
    remote: "/backup"
`)
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := config.SaveRaw(path, []byte("alist: [")); err == nil {
		t.Fatal("expected invalid YAML to fail")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatalf("config was overwritten\n got: %q\nwant: %q", got, original)
	}
}

func TestSelectTasks(t *testing.T) {
	tasks := []config.Task{{Name: "a"}, {Name: "b"}}
	all, err := config.SelectTasks(tasks, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("expected all tasks, got %d", len(all))
	}
	one, err := config.SelectTasks(tasks, "b", false)
	if err != nil {
		t.Fatal(err)
	}
	if one[0].Name != "b" {
		t.Fatalf("unexpected task: %v", one)
	}
	if _, err := config.SelectTasks(tasks, "", false); err == nil {
		t.Fatal("expected error when multiple tasks and no selector")
	}
}

func TestBuildRcloneArgs(t *testing.T) {
	cfg := config.Config{
		Rclone: config.RcloneConfig{
			Remote:     "alist_baidu",
			ConfigFile: ".alist-sync/rclone.conf",
			Transfers:  4,
			Checkers:   8,
			Excludes:   []string{"**/.venv/**", "**/.git/**"},
		},
	}
	task := config.Task{
		Name:     "documents",
		Local:    "D:/My Documents",
		Remote:   "/BaiduPanBackup/Documents",
		Excludes: []string{"private/**"},
	}

	got := rclone.BuildArgs("dry-run", cfg, task)
	want := []string{
		"sync",
		config.ToNativePath("D:/My Documents"),
		"alist_baidu:BaiduPanBackup/Documents",
		"--config", config.ToNativePath(".alist-sync/rclone.conf"),
		"--transfers", "4",
		"--checkers", "8",
		"--retries", "8",
		"--low-level-retries", "20",
		"--retries-sleep", "5s",
		"--progress",
		"--exclude", "**/.venv/**",
		"--exclude", "**/.git/**",
		"--exclude", "private/**",
		"--dry-run", "--combined", "-",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dry-run args mismatch\n got: %#v\nwant: %#v", got, want)
	}

	update := rclone.BuildArgs("update", cfg, task)
	if update[0] != "copy" {
		t.Fatalf("update should use copy, got %s", update[0])
	}
	sync := rclone.BuildArgs("sync", cfg, task)
	if sync[0] != "sync" {
		t.Fatalf("sync should use sync, got %s", sync[0])
	}
}

func TestFindUnsupportedUploadNamesReportsFullwidthColon(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "subject\uff1aKAG52352.eml")
	if err := os.WriteFile(bad, []byte("mail"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "normal.eml"), []byte("mail"), 0o644); err != nil {
		t.Fatal(err)
	}

	problems, err := filename.FindUnsupportedUploadNames([]config.Task{{Name: "documents", Local: dir}}, filename.MaxProblems)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 {
		t.Fatalf("expected one problem, got %#v", problems)
	}
	if problems[0].Task != "documents" || !strings.Contains(problems[0].Why, "U+FF1A") {
		t.Fatalf("unexpected problem: %#v", problems[0])
	}
}

func TestDependencyStatusFindsLocalTool(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(".alist-sync", "tools", deps.ExeName("rclone")), []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}
	st := deps.CheckTool("rclone")
	if !st.Available || st.Source != "local" {
		t.Fatalf("expected local rclone, got %#v", st)
	}
}

func TestSyncWithoutYesRunsSync(t *testing.T) {
	dir := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

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
	if err := os.WriteFile(filepath.Join(".alist-sync", "tools", deps.ExeName("rclone")), []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}
	localDir := filepath.ToSlash(t.TempDir())
	cfgPath := filepath.Join(dir, "config.yaml")
	err = os.WriteFile(cfgPath, []byte(`alist:
  url: "`+server.URL+`"
  username: "admin"
rclone:
  remote: "alist_baidu"
  config_file: ".alist-sync/rclone.conf"
tasks:
  - name: "documents"
    local: "`+localDir+`"
    remote: "/backup"
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ALIST_PASSWORD", "secret")
	var calls [][]string
	r := Runner{
		stdout: os.Stdout,
		stderr: os.Stderr,
		output: func(name string, args ...string) (string, error) {
			return "obscured-secret", nil
		},
		exec: func(name string, args ...string) error {
			calls = append(calls, append([]string{name}, args...))
			return nil
		},
	}
	err = r.cmdTransfer("sync", []string{"--config", cfgPath, "documents"})
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}
	if len(calls) != 1 || calls[0][1] != "sync" || containsArg(calls[0], "--dry-run") {
		t.Fatalf("expected one real sync call, got %#v", calls)
	}
}

func TestUpdateStartsAListWhenConfiguredAndWaitsUntilReady(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(".alist-sync", "tools", deps.ExeName("rclone")), []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}

	var ready atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ready.Load() {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	localDir := filepath.ToSlash(t.TempDir())
	cfgPath := filepath.Join(dir, "config.yaml")
	err = os.WriteFile(cfgPath, []byte(`alist:
  url: "`+server.URL+`"
  username: "admin"
  server_command: '"C:\alist\alist.exe" server'
  startup_timeout_seconds: 1
rclone:
  remote: "alist_baidu"
  config_file: ".alist-sync/rclone.conf"
tasks:
  - name: "documents"
    local: "`+localDir+`"
    remote: "/backup"
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ALIST_PASSWORD", "secret")
	var started [][]string
	var calls [][]string
	r := Runner{
		stdout: os.Stdout,
		stderr: os.Stderr,
		output: func(name string, args ...string) (string, error) {
			return "obscured-secret", nil
		},
		start: func(name string, args ...string) error {
			started = append(started, append([]string{name}, args...))
			ready.Store(true)
			return nil
		},
		exec: func(name string, args ...string) error {
			calls = append(calls, append([]string{name}, args...))
			return nil
		},
	}
	if err := r.cmdTransfer("update", []string{"--config", cfgPath, "documents"}); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if len(started) != 1 {
		t.Fatalf("expected AList to start once, got %#v", started)
	}
	if started[0][0] != `C:\alist\alist.exe` || started[0][1] != "server" {
		t.Fatalf("unexpected start command: %#v", started)
	}
	if len(calls) != 1 || calls[0][1] != "copy" {
		t.Fatalf("expected one rclone copy call, got %#v", calls)
	}
}

func TestSplitCommandPreservesWindowsBackslashes(t *testing.T) {
	name, args, err := alist.SplitCommand(`"C:\Program Files\alist\alist.exe" server --data "D:\alist data"`)
	if err != nil {
		t.Fatal(err)
	}
	if name != `C:\Program Files\alist\alist.exe` {
		t.Fatalf("unexpected name: %s", name)
	}
	want := []string{"server", "--data", `D:\alist data`}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args mismatch\n got: %#v\nwant: %#v", args, want)
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
	got := config.ToNativePath(".alist-sync/rclone.conf")
	if runtime.GOOS == "windows" && strings.Contains(got, "/") {
		t.Fatalf("expected native windows separators, got %s", got)
	}
}
