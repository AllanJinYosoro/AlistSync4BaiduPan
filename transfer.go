package main

import (
	"flag"
	"fmt"
)

func (r Runner) cmdTransfer(mode string, args []string) error {
	fs := flag.NewFlagSet(mode, flag.ContinueOnError)
	fs.SetOutput(r.stderr)
	configPath := fs.String("config", defaultConfigPath, "config file path")
	all := fs.Bool("all", false, "run all tasks")
	fs.Bool("yes", false, "accepted for backward compatibility; sync runs by default")
	if err := fs.Parse(args); err != nil {
		return err
	}
	selector, err := transferSelector(fs.Args())
	if err != nil {
		return err
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
	if problems, err := findUnsupportedUploadNames(tasks, maxNameProblems); err != nil {
		return err
	} else if len(problems) > 0 {
		return fmt.Errorf("%s", formatNameProblems(problems))
	}
	if err := r.ensureAListReady(cfg); err != nil {
		return err
	}
	if err := r.ensureRcloneConfig(cfg); err != nil {
		return err
	}
	rclonePath, err := findTool("rclone")
	if err != nil {
		return err
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

func transferSelector(args []string) (string, error) {
	selector := ""
	for _, arg := range args {
		if arg == "--yes" {
			continue
		}
		if selector != "" {
			return "", fmt.Errorf("unexpected argument %q", arg)
		}
		selector = arg
	}
	return selector, nil
}
