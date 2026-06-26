package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

func (r Runner) ensureAListReady(cfg Config) error {
	if isAListReachable(cfg.AList.URL) {
		fmt.Fprintln(r.stdout, "AList is reachable:", strings.TrimRight(cfg.AList.URL, "/"))
		return nil
	}
	if strings.TrimSpace(cfg.AList.ServerCommand) == "" {
		return fmt.Errorf("AList is not reachable at %s and alist.server_command is not configured", cfg.AList.URL)
	}

	command, args, err := splitCommand(cfg.AList.ServerCommand)
	if err != nil {
		return fmt.Errorf("invalid alist.server_command: %w", err)
	}
	if r.start == nil {
		r.start = startBackgroundProcess
	}
	fmt.Fprintln(r.stdout, "AList is not reachable; starting:", cfg.AList.ServerCommand)
	if err := r.start(command, args...); err != nil {
		return fmt.Errorf("start AList failed: %w", err)
	}
	timeout := time.Duration(cfg.AList.StartupTimeoutSeconds) * time.Second
	if err := waitForAList(cfg.AList.URL, timeout); err != nil {
		return err
	}
	fmt.Fprintln(r.stdout, "AList is ready:", strings.TrimRight(cfg.AList.URL, "/"))
	return nil
}

func isAListReachable(rawURL string) bool {
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(strings.TrimRight(rawURL, "/"))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < 500
}

func waitForAList(rawURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if isAListReachable(rawURL) {
			return nil
		}
		if timeout == 0 || time.Now().After(deadline) {
			return fmt.Errorf("AList did not become reachable at %s within %s", rawURL, timeout)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func splitCommand(command string) (string, []string, error) {
	var parts []string
	var current strings.Builder
	var quote rune
	for _, ch := range command {
		if quote != 0 {
			if ch == quote {
				quote = 0
				continue
			}
			current.WriteRune(ch)
			continue
		}
		if ch == '\'' || ch == '"' {
			quote = ch
			continue
		}
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(ch)
	}
	if quote != 0 {
		return "", nil, errors.New("unterminated quote")
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	if len(parts) == 0 {
		return "", nil, errors.New("empty command")
	}
	return parts[0], parts[1:], nil
}

func startBackgroundProcess(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
