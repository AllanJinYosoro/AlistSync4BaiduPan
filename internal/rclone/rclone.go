package rclone

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"bdp-sync/internal/config"
	"bdp-sync/internal/deps"
	"bdp-sync/internal/proc"
)

type OutputFunc func(name string, args ...string) (string, error)

func EnsureConfig(cfg config.Config, output OutputFunc, stdout io.Writer) error {
	password := os.Getenv(cfg.AList.PasswordEnv)
	if password == "" {
		return fmt.Errorf("environment variable %s is required for AList WebDAV password", cfg.AList.PasswordEnv)
	}
	rclonePath, err := deps.FindTool("rclone")
	if err != nil {
		return err
	}
	if output == nil {
		output = runOutput
	}
	obscured, err := output(rclonePath, "obscure", password)
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
	fmt.Fprintln(stdout, "wrote rclone config", rcloneConfig)
	return nil
}

func BuildArgs(mode string, cfg config.Config, task config.Task) []string {
	command := "sync"
	if mode == "update" {
		command = "copy"
	}
	args := []string{
		command,
		config.ToNativePath(task.Local),
		cfg.Rclone.Remote + ":" + config.TrimRemotePath(task.Remote),
		"--config", cfg.RcloneConfigPath(),
		"--transfers", strconv.Itoa(cfg.Rclone.Transfers),
		"--checkers", strconv.Itoa(cfg.Rclone.Checkers),
		"--retries", strconv.Itoa(cfg.Rclone.Retries),
		"--low-level-retries", strconv.Itoa(cfg.Rclone.LowLevelRetries),
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

func runOutput(name string, args ...string) (string, error) {
	cmd := proc.Command(name, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
