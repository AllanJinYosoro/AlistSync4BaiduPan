package config

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultPath   = "config.yaml"
	LocalStateDir = ".alist-sync"
	ToolsDir      = ".alist-sync/tools"
)

type Config struct {
	AList  AListConfig  `yaml:"alist"`
	Rclone RcloneConfig `yaml:"rclone"`
	Tasks  []Task       `yaml:"tasks"`
}

type AListConfig struct {
	URL                   string `yaml:"url"`
	Username              string `yaml:"username"`
	PasswordEnv           string `yaml:"password_env"`
	ServerCommand         string `yaml:"server_command"`
	StartupTimeoutSeconds int    `yaml:"startup_timeout_seconds"`
}

type RcloneConfig struct {
	Remote     string   `yaml:"remote"`
	ConfigFile string   `yaml:"config_file"`
	Transfers  int      `yaml:"transfers"`
	Checkers   int      `yaml:"checkers"`
	Excludes   []string `yaml:"excludes"`
}

type Task struct {
	Name     string   `yaml:"name"`
	Local    string   `yaml:"local"`
	Remote   string   `yaml:"remote"`
	Excludes []string `yaml:"excludes"`
}

func Load(path string) (Config, error) {
	if err := LoadDotEnv(".env"); err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	return ParseYAML(data)
}

func ParseYAML(data []byte) (Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	ApplyDefaults(&cfg)
	return cfg, nil
}

func ParseAndValidateYAML(data []byte) (Config, error) {
	cfg, err := ParseYAML(data)
	if err != nil {
		return Config{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func ApplyDefaults(cfg *Config) {
	if cfg.AList.URL == "" {
		cfg.AList.URL = "http://127.0.0.1:5244"
	}
	if cfg.AList.PasswordEnv == "" {
		cfg.AList.PasswordEnv = "ALIST_PASSWORD"
	}
	if cfg.AList.StartupTimeoutSeconds == 0 {
		cfg.AList.StartupTimeoutSeconds = 30
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
	if c.AList.StartupTimeoutSeconds < 0 {
		return errors.New("alist.startup_timeout_seconds must be greater than or equal to 0")
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
	return ToNativePath(c.Rclone.ConfigFile)
}

func (c Config) Clone() Config {
	clone := c
	clone.Rclone.Excludes = append([]string(nil), c.Rclone.Excludes...)
	clone.Tasks = append([]Task(nil), c.Tasks...)
	for i := range clone.Tasks {
		clone.Tasks[i].Excludes = append([]string(nil), c.Tasks[i].Excludes...)
	}
	return clone
}

func Save(path string, cfg Config) error {
	data, err := FormatYAML(cfg)
	if err != nil {
		return err
	}
	return SaveRaw(path, data)
}

func SaveRaw(path string, data []byte) error {
	if _, err := ParseAndValidateYAML(data); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func FormatYAML(cfg Config) ([]byte, error) {
	ApplyDefaults(&cfg)
	return yaml.Marshal(cfg)
}

func LoadText(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func TaskNames(cfg Config) []string {
	names := make([]string, 0, len(cfg.Tasks))
	for _, task := range cfg.Tasks {
		if task.Name != "" {
			names = append(names, task.Name)
		}
	}
	return names
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

func TrimRemotePath(path string) string {
	return strings.TrimPrefix(strings.ReplaceAll(path, "\\", "/"), "/")
}

func ToNativePath(path string) string {
	if runtime.GOOS == "windows" {
		return filepath.FromSlash(path)
	}
	return path
}

func EnsureLocalDirs() error {
	if err := os.MkdirAll(ToNativePath(LocalStateDir), 0o755); err != nil {
		return err
	}
	return os.MkdirAll(ToNativePath(ToolsDir), 0o755)
}

func WriteSampleIfMissing(path string) (bool, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(path, []byte(Sample()), 0o644); err != nil {
			return false, err
		}
		return true, nil
	} else if err != nil {
		return false, err
	}
	return false, nil
}

func Sample() string {
	return sampleConfig
}

func LoadDotEnv(path string) error {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	return ParseDotEnv(file)
}

func ParseDotEnv(r io.Reader) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" || os.Getenv(key) != "" {
			continue
		}
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	return scanner.Err()
}

const sampleConfig = `alist:
  url: "http://127.0.0.1:5244"
  username: "admin"
  password_env: "ALIST_PASSWORD"
  server_command: ".alist-sync/tools/alist.exe server"
  startup_timeout_seconds: 30

rclone:
  remote: "alist_baidu"
  config_file: ".alist-sync/rclone.conf"
  transfers: 4
  checkers: 8
  excludes:
    - "**/.venv/**"
    - "**/__pycache__/**"
    - "**/.git/**"

tasks:
  - name: "documents"
    local: "D:/Documents"
    remote: "/BaiduPanBackup/Documents"
    excludes:
      - "private/**"
`
