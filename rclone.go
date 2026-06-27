package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func (r Runner) ensureRcloneConfig(cfg Config) error {
	password := os.Getenv(cfg.AList.PasswordEnv)
	if password == "" {
		return fmt.Errorf("environment variable %s is required for AList WebDAV password", cfg.AList.PasswordEnv)
	}
	rclonePath, err := findTool("rclone")
	if err != nil {
		return err
	}
	obscured, err := r.runOutput(rclonePath, "obscure", password)
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

func (r Runner) runOutput(name string, args ...string) (string, error) {
	if r.output != nil {
		return r.output(name, args...)
	}
	return runOutput(name, args...)
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
		"--retries", "8",
		"--low-level-retries", "20",
		"--retries-sleep", "5s",
		"--progress",
	}
	for _, exclude := range cfg.Rclone.Excludes {
		args = append(args, "--exclude", exclude)
	}
	for _, exclude := range task.Excludes {
		args = append(args, "--exclude", exclude)
	}
	if mode == "dry-run" {
		args = append(args, "--dry-run", "--combined", "-")
	}
	return args
}
