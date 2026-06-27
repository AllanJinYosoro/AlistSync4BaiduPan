package alist

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"bdp-sync/internal/config"
)

type Starter func(name string, args ...string) error
type ManagedStarter func(name string, args ...string) (func() error, error)

func EnsureReady(cfg config.Config, start Starter, stdout io.Writer) error {
	if IsReachable(cfg.AList.URL) {
		fmt.Fprintln(stdout, "AList is reachable:", strings.TrimRight(cfg.AList.URL, "/"))
		return nil
	}
	if strings.TrimSpace(cfg.AList.ServerCommand) == "" {
		return fmt.Errorf("AList is not reachable at %s and alist.server_command is not configured", cfg.AList.URL)
	}

	command, args, err := SplitCommand(cfg.AList.ServerCommand)
	if err != nil {
		return fmt.Errorf("invalid alist.server_command: %w", err)
	}
	if start == nil {
		start = StartBackgroundProcess
	}
	fmt.Fprintln(stdout, "AList is not reachable; starting:", cfg.AList.ServerCommand)
	if err := start(command, args...); err != nil {
		return fmt.Errorf("start AList failed: %w", err)
	}
	timeout := time.Duration(cfg.AList.StartupTimeoutSeconds) * time.Second
	if err := Wait(cfg.AList.URL, timeout); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "AList is ready:", strings.TrimRight(cfg.AList.URL, "/"))
	return nil
}

func EnsureReadyManaged(cfg config.Config, start ManagedStarter, stdout io.Writer) (func() error, error) {
	if IsReachable(cfg.AList.URL) {
		fmt.Fprintln(stdout, "AList is reachable:", strings.TrimRight(cfg.AList.URL, "/"))
		return nil, nil
	}
	if strings.TrimSpace(cfg.AList.ServerCommand) == "" {
		return nil, fmt.Errorf("AList is not reachable at %s and alist.server_command is not configured", cfg.AList.URL)
	}

	command, args, err := SplitCommand(cfg.AList.ServerCommand)
	if err != nil {
		return nil, fmt.Errorf("invalid alist.server_command: %w", err)
	}
	if start == nil {
		start = StartManagedProcess
	}
	fmt.Fprintln(stdout, "AList is not reachable; starting for doctor:", cfg.AList.ServerCommand)
	stop, err := start(command, args...)
	if err != nil {
		return nil, fmt.Errorf("start AList failed: %w", err)
	}
	timeout := time.Duration(cfg.AList.StartupTimeoutSeconds) * time.Second
	if err := Wait(cfg.AList.URL, timeout); err != nil {
		if stop != nil {
			_ = stop()
		}
		return nil, err
	}
	fmt.Fprintln(stdout, "AList is ready:", strings.TrimRight(cfg.AList.URL, "/"))
	return stop, nil
}

func IsReachable(rawURL string) bool {
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(strings.TrimRight(rawURL, "/"))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < 500
}

func Wait(rawURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if IsReachable(rawURL) {
			return nil
		}
		if timeout == 0 || time.Now().After(deadline) {
			return fmt.Errorf("AList did not become reachable at %s within %s", rawURL, timeout)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func SplitCommand(command string) (string, []string, error) {
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

func StartBackgroundProcess(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func StartManagedProcess(name string, args ...string) (func() error, error) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return func() error {
		if cmd.Process == nil {
			return nil
		}
		if err := cmd.Process.Kill(); err != nil {
			return err
		}
		if err := cmd.Wait(); err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				return nil
			}
			return err
		}
		return nil
	}, nil
}
