package main

import (
	"errors"
	"fmt"
	"strings"
)

func taskNames(cfg Config) []string {
	names := make([]string, 0, len(cfg.Tasks))
	for _, task := range cfg.Tasks {
		if task.Name != "" {
			names = append(names, task.Name)
		}
	}
	return names
}

func guiCommandArgs(action, configPath, selectedTask string, all bool) ([]string, error) {
	if strings.TrimSpace(configPath) == "" {
		configPath = defaultConfigPath
	}

	switch action {
	case "doctor":
		return []string{"doctor", "--config", configPath}, nil
	case "dry-run", "update", "sync":
		args := []string{action, "--config", configPath}
		if all {
			return append(args, "--all"), nil
		}
		if strings.TrimSpace(selectedTask) == "" {
			return nil, errors.New("select a task or enable all tasks")
		}
		return append(args, selectedTask), nil
	default:
		return nil, fmt.Errorf("unknown action %q", action)
	}
}
