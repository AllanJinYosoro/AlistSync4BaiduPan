package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

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
	URL                   string `yaml:"url"`
	Username              string `yaml:"username"`
	PasswordEnv           string `yaml:"password_env"`
	ServerCommand         string `yaml:"server_command"`
	StartupTimeoutSeconds int    `yaml:"startup_timeout_seconds"`
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
  server_command: ".alist-sync/tools/alist.exe server"
  startup_timeout_seconds: 30

rclone:
  remote: "alist_baidu"
  config_file: ".alist-sync/rclone.conf"
  transfers: 4
  checkers: 8

tasks:
  - name: "documents"
    local: "D:/Documents"
    remote: "/BaiduPanBackup/Documents"
`
