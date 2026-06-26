package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultConfigPath = "config.yaml"
	localStateDir     = ".alist-sync"
	toolsDir          = ".alist-sync/tools"
)

type Config struct {
	AList  AListConfig  `yaml:"alist"`
	Rclone RcloneConfig `yaml:"rclone"`
	Tasks  []Task       `yaml:"tasks"`
}

type AListConfig struct {
	URL         string `yaml:"url"`
	Username    string `yaml:"username"`
	PasswordEnv string `yaml:"password_env"`
}

type RcloneConfig struct {
	Remote     string `yaml:"remote"`
	ConfigFile string `yaml:"config_file"`
	Transfers  int    `yaml:"transfers"`
	Checkers   int    `yaml:"checkers"`
}

type Task struct {
	Name   string `yaml:"name"`
	Local  string `yaml:"local"`
	Remote string `yaml:"remote"`
}

type Runner struct {
	stdout io.Writer
	stderr io.Writer
	exec   func(name string, args ...string) error
}

func main() {
	r := Runner{
		stdout: os.Stdout,
		stderr: os.Stderr,
		exec: func(name string, args ...string) error {
			cmd := exec.Command(name, args...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		},
	}
	if err := r.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func (r Runner) Run(args []string) error {
	if len(args) == 0 {
		printUsage(r.stdout)
		return nil
	}

	switch args[0] {
	case "init":
		return r.cmdInit(args[1:])
	case "setup":
		return r.cmdSetup(args[1:])
	case "doctor":
		return r.cmdDoctor(args[1:])
	case "dry-run", "sync", "update":
		return r.cmdTransfer(args[0], args[1:])
	case "help", "-h", "--help":
		printUsage(r.stdout)
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `alist-sync - AList + rclone manual backup sync for Baidu Netdisk

Usage:
  alist-sync init
  alist-sync setup deps
  alist-sync setup rclone
  alist-sync doctor
  alist-sync dry-run [task|--all]
  alist-sync sync [task|--all] --yes
  alist-sync update [task|--all]

Global flags:
  --config PATH     Config file path, defaults to config.yaml
`)
}

func (r Runner) cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(r.stderr)
	configPath := fs.String("config", defaultConfigPath, "config file path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := os.MkdirAll(toNativePath(localStateDir), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(toNativePath(toolsDir), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(*configPath); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(*configPath, []byte(sampleConfig), 0o644); err != nil {
			return err
		}
		fmt.Fprintln(r.stdout, "created", *configPath)
	} else if err != nil {
		return err
	} else {
		fmt.Fprintln(r.stdout, "kept existing", *configPath)
	}
	if err := ensureGitignore(); err != nil {
		return err
	}
	fmt.Fprintln(r.stdout, "created local state directory", localStateDir)
	return nil
}

func (r Runner) cmdSetup(args []string) error {
	if len(args) == 0 {
		return errors.New("setup requires subcommand: deps or rclone")
	}
	switch args[0] {
	case "deps":
		return r.cmdSetupDeps(args[1:])
	case "rclone":
		return r.cmdSetupRclone(args[1:])
	default:
		return fmt.Errorf("unknown setup subcommand %q", args[0])
	}
}

func (r Runner) cmdSetupDeps(args []string) error {
	fs := flag.NewFlagSet("setup deps", flag.ContinueOnError)
	fs.SetOutput(r.stderr)
	force := fs.Bool("force", false, "download even when tool exists")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := os.MkdirAll(toNativePath(toolsDir), 0o755); err != nil {
		return err
	}
	if err := ensureTool("rclone", *force, r.stdout); err != nil {
		return err
	}
	if err := ensureTool("alist", *force, r.stdout); err != nil {
		return err
	}
	return nil
}

func (r Runner) cmdSetupRclone(args []string) error {
	fs := flag.NewFlagSet("setup rclone", flag.ContinueOnError)
	fs.SetOutput(r.stderr)
	configPath := fs.String("config", defaultConfigPath, "config file path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := LoadConfig(*configPath)
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	password := os.Getenv(cfg.AList.PasswordEnv)
	if password == "" {
		return fmt.Errorf("environment variable %s is required for AList WebDAV password", cfg.AList.PasswordEnv)
	}
	rclonePath, err := findTool("rclone")
	if err != nil {
		return err
	}
	obscured, err := runOutput(rclonePath, "obscure", password)
	if err != nil {
		return fmt.Errorf("rclone obscure failed: %w", err)
	}
	webdavURL := strings.TrimRight(cfg.AList.URL, "/") + "/dav/"
	conf := fmt.Sprintf("[%s]\ntype = webdav\nurl = %s\nvendor = other\nuser = %s\npass = %s\n\n",
		cfg.Rclone.Remote,
		webdavURL,
		cfg.AList.Username,
		strings.TrimSpace(obscured),
	)
	rcloneConfig := cfg.RcloneConfigPath()
	if err := os.MkdirAll(filepath.Dir(rcloneConfig), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(rcloneConfig, []byte(conf), 0o600); err != nil {
		return err
	}
	fmt.Fprintln(r.stdout, "wrote rclone config", rcloneConfig)
	return nil
}

func (r Runner) cmdDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(r.stderr)
	configPath := fs.String("config", defaultConfigPath, "config file path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	checks := []doctorCheck{
		checkCommand("git", "--version"),
		checkCommand("go", "version"),
		checkTool("alist"),
		checkTool("rclone"),
	}
	cfg, err := LoadConfig(*configPath)
	if err != nil {
		checks = append(checks, doctorCheck{"config", false, err.Error()})
	} else {
		validateErr := cfg.Validate()
		checks = append(checks, doctorCheck{"config", validateErr == nil, errString(validateErr)})
		checks = append(checks, checkPasswordEnv(cfg))
		checks = append(checks, checkAListURL(cfg))
		checks = append(checks, checkRcloneRemote(cfg))
	}

	failed := false
	for _, c := range checks {
		if c.ok {
			fmt.Fprintf(r.stdout, "[OK]   %s\n", c.name)
			continue
		}
		failed = true
		fmt.Fprintf(r.stdout, "[FAIL] %s: %s\n", c.name, c.detail)
	}
	if failed {
		return errors.New("doctor found problems")
	}
	return nil
}

func (r Runner) cmdTransfer(mode string, args []string) error {
	fs := flag.NewFlagSet(mode, flag.ContinueOnError)
	fs.SetOutput(r.stderr)
	configPath := fs.String("config", defaultConfigPath, "config file path")
	all := fs.Bool("all", false, "run all tasks")
	yes := fs.Bool("yes", false, "confirm destructive sync")
	if err := fs.Parse(args); err != nil {
		return err
	}
	selector := ""
	if fs.NArg() > 0 {
		selector = fs.Arg(0)
	}
	cfg, err := LoadConfig(*configPath)
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	tasks, err := SelectTasks(cfg.Tasks, selector, *all)
	if err != nil {
		return err
	}
	rclonePath, err := findTool("rclone")
	if err != nil {
		return err
	}
	if mode == "sync" && !*yes {
		fmt.Fprintln(r.stdout, "sync can delete remote-only files. Previewing planned changes first:")
		for _, task := range tasks {
			args := BuildRcloneArgs("dry-run", cfg, task)
			fmt.Fprintf(r.stdout, "\n==> %s: %s -> %s:%s\n", task.Name, task.Local, cfg.Rclone.Remote, trimRemotePath(task.Remote))
			if err := r.exec(rclonePath, args...); err != nil {
				return fmt.Errorf("dry-run failed for task %s: %w", task.Name, err)
			}
		}
		return errors.New("review the preview, then rerun sync with --yes to apply changes")
	}
	for _, task := range tasks {
		args := BuildRcloneArgs(mode, cfg, task)
		fmt.Fprintf(r.stdout, "\n==> %s: %s -> %s:%s\n", task.Name, task.Local, cfg.Rclone.Remote, trimRemotePath(task.Remote))
		if err := r.exec(rclonePath, args...); err != nil {
			return fmt.Errorf("%s failed for task %s: %w", mode, task.Name, err)
		}
	}
	return nil
}

func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	if cfg.AList.URL == "" {
		cfg.AList.URL = "http://127.0.0.1:5244"
	}
	if cfg.AList.PasswordEnv == "" {
		cfg.AList.PasswordEnv = "ALIST_PASSWORD"
	}
	if cfg.Rclone.Remote == "" {
		cfg.Rclone.Remote = "alist_baidu"
	}
	if cfg.Rclone.ConfigFile == "" {
		cfg.Rclone.ConfigFile = ".alist-sync/rclone.conf"
	}
	if cfg.Rclone.Transfers == 0 {
		cfg.Rclone.Transfers = 4
	}
	if cfg.Rclone.Checkers == 0 {
		cfg.Rclone.Checkers = 8
	}
	return cfg, nil
}

func (c Config) Validate() error {
	var missing []string
	if c.AList.URL == "" {
		missing = append(missing, "alist.url")
	}
	if c.AList.Username == "" {
		missing = append(missing, "alist.username")
	}
	if c.AList.PasswordEnv == "" {
		missing = append(missing, "alist.password_env")
	}
	if c.Rclone.Remote == "" {
		missing = append(missing, "rclone.remote")
	}
	if c.Rclone.ConfigFile == "" {
		missing = append(missing, "rclone.config_file")
	}
	if len(c.Tasks) == 0 {
		missing = append(missing, "tasks")
	}
	seen := map[string]bool{}
	for i, task := range c.Tasks {
		prefix := "tasks[" + strconv.Itoa(i) + "]"
		if task.Name == "" {
			missing = append(missing, prefix+".name")
		}
		if task.Local == "" {
			missing = append(missing, prefix+".local")
		}
		if task.Remote == "" {
			missing = append(missing, prefix+".remote")
		}
		if task.Name != "" && seen[task.Name] {
			return fmt.Errorf("duplicate task name %q", task.Name)
		}
		seen[task.Name] = true
	}
	if len(missing) > 0 {
		return errors.New("missing required config: " + strings.Join(missing, ", "))
	}
	if _, err := url.ParseRequestURI(c.AList.URL); err != nil {
		return fmt.Errorf("invalid alist.url: %w", err)
	}
	return nil
}

func (c Config) RcloneConfigPath() string {
	return toNativePath(c.Rclone.ConfigFile)
}

func SelectTasks(tasks []Task, selector string, all bool) ([]Task, error) {
	if all {
		if selector != "" {
			return nil, errors.New("use either a task name or --all, not both")
		}
		return tasks, nil
	}
	if selector == "" {
		if len(tasks) == 1 {
			return tasks, nil
		}
		return nil, errors.New("multiple tasks configured; pass a task name or --all")
	}
	for _, task := range tasks {
		if task.Name == selector {
			return []Task{task}, nil
		}
	}
	return nil, fmt.Errorf("task %q not found", selector)
}

func BuildRcloneArgs(mode string, cfg Config, task Task) []string {
	command := "sync"
	if mode == "update" {
		command = "copy"
	}
	args := []string{
		command,
		toNativePath(task.Local),
		cfg.Rclone.Remote + ":" + trimRemotePath(task.Remote),
		"--config", cfg.RcloneConfigPath(),
		"--transfers", strconv.Itoa(cfg.Rclone.Transfers),
		"--checkers", strconv.Itoa(cfg.Rclone.Checkers),
		"--progress",
	}
	if mode == "dry-run" {
		args = append(args, "--dry-run", "--combined", "-")
	}
	return args
}

func ensureGitignore() error {
	const managed = ".alist-sync/\nconfig.local.yaml\n*.log\n*.exe\n"
	existing, err := os.ReadFile(".gitignore")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	text := string(existing)
	var add []string
	for _, line := range strings.Split(strings.TrimSpace(managed), "\n") {
		if !containsLine(text, line) {
			add = append(add, line)
		}
	}
	if len(add) == 0 {
		return nil
	}
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	text += strings.Join(add, "\n") + "\n"
	return os.WriteFile(".gitignore", []byte(text), 0o644)
}

func containsLine(text, line string) bool {
	for _, existing := range strings.Split(text, "\n") {
		if strings.TrimSpace(existing) == line {
			return true
		}
	}
	return false
}

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
	return "", fmt.Errorf("%s not found; run `alist-sync setup deps` first", name)
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

type doctorCheck struct {
	name   string
	ok     bool
	detail string
}

func checkCommand(name string, args ...string) doctorCheck {
	err := exec.Command(name, args...).Run()
	return doctorCheck{name: name, ok: err == nil, detail: errString(err)}
}

func checkTool(name string) doctorCheck {
	p, err := findTool(name)
	if err != nil {
		return doctorCheck{name: name, ok: false, detail: err.Error()}
	}
	return doctorCheck{name: name, ok: true, detail: p}
}

func checkPasswordEnv(cfg Config) doctorCheck {
	value := os.Getenv(cfg.AList.PasswordEnv)
	return doctorCheck{
		name:   cfg.AList.PasswordEnv,
		ok:     value != "",
		detail: "environment variable is empty",
	}
}

func checkAListURL(cfg Config) doctorCheck {
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(strings.TrimRight(cfg.AList.URL, "/"))
	if err != nil {
		return doctorCheck{name: "alist.url", ok: false, detail: err.Error()}
	}
	defer resp.Body.Close()
	return doctorCheck{name: "alist.url", ok: resp.StatusCode < 500, detail: resp.Status}
}

func checkRcloneRemote(cfg Config) doctorCheck {
	rclonePath, err := findTool("rclone")
	if err != nil {
		return doctorCheck{name: "rclone remote", ok: false, detail: err.Error()}
	}
	err = exec.Command(rclonePath, "lsd", cfg.Rclone.Remote+":", "--config", cfg.RcloneConfigPath()).Run()
	return doctorCheck{name: "rclone remote", ok: err == nil, detail: errString(err)}
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

func trimRemotePath(path string) string {
	return strings.TrimPrefix(strings.ReplaceAll(path, "\\", "/"), "/")
}

func toNativePath(path string) string {
	if runtime.GOOS == "windows" {
		return filepath.FromSlash(path)
	}
	return path
}

const sampleConfig = `alist:
  url: "http://127.0.0.1:5244"
  username: "admin"
  password_env: "ALIST_PASSWORD"

rclone:
  remote: "alist_baidu"
  config_file: ".alist-sync/rclone.conf"
  transfers: 4
  checkers: 8

tasks:
  - name: "documents"
    local: "D:/Documents"
    remote: "/百度网盘备份/Documents"
`
