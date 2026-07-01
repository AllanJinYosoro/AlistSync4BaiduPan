package runner

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bdp-sync/internal/alist"
	"bdp-sync/internal/config"
	"bdp-sync/internal/deps"
	"bdp-sync/internal/filename"
	"bdp-sync/internal/proc"
	"bdp-sync/internal/rclone"
)

func (r Runner) cmdTransfer(mode string, args []string) error {
	fs := flag.NewFlagSet(mode, flag.ContinueOnError)
	fs.SetOutput(r.stderr)
	configPath := fs.String("config", config.DefaultPath, "config file path")
	all := fs.Bool("all", false, "run all tasks")
	scopedPath := fs.String("path", "", "local file or folder path to transfer")
	fs.Bool("yes", false, "accepted for backward compatibility; sync runs by default")
	if err := fs.Parse(args); err != nil {
		return err
	}
	selector, err := transferSelector(fs.Args())
	if err != nil {
		return err
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	tasks, err := config.SelectTasks(cfg.Tasks, selector, *all)
	if err != nil {
		return err
	}
	if strings.TrimSpace(*scopedPath) != "" {
		if *all {
			return fmt.Errorf("--path requires one selected task, not --all")
		}
		tasks[0], err = scopedTask(mode, tasks[0], *scopedPath)
		if err != nil {
			return err
		}
	}
	if problems, err := filename.FindUnsupportedUploadNamesContext(r.context(), tasks, filename.MaxProblems); err != nil {
		return err
	} else if len(problems) > 0 {
		return fmt.Errorf("%s", filename.FormatProblems(problems))
	}
	zeroByteFiles, err := filename.FindZeroByteFilesContext(r.context(), tasks)
	if err != nil {
		return err
	}
	zeroByteExcludes := map[string][]string{}
	for _, file := range zeroByteFiles {
		zeroByteExcludes[file.Task] = append(zeroByteExcludes[file.Task], file.Exclude)
	}
	if err := r.context().Err(); err != nil {
		return err
	}
	if err := alist.EnsureReadyContext(r.context(), cfg, r.start, r.stdout); err != nil {
		return err
	}
	if err := rclone.EnsureConfig(cfg, r.runOutput, r.stdout); err != nil {
		return err
	}
	rclonePath, err := deps.FindTool("rclone")
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if err := r.context().Err(); err != nil {
			return err
		}
		if excludes := zeroByteExcludes[task.Name]; len(excludes) > 0 {
			fmt.Fprintf(r.stdout, "skipping %d zero-byte file(s) for task %s\n", len(excludes), task.Name)
			task.Excludes = append(append([]string(nil), task.Excludes...), excludes...)
		}
		args := rclone.BuildArgs(mode, cfg, task)
		fmt.Fprintf(r.stdout, "\n==> %s: %s -> %s:%s\n", task.Name, task.Local, cfg.Rclone.Remote, config.TrimRemotePath(task.Remote))
		if err := r.exec(rclonePath, args...); err != nil {
			return fmt.Errorf("%s failed for task %s: %w", mode, task.Name, err)
		}
	}
	if mode == "dry-run" {
		fmt.Fprintf(r.stdout, "\nDry run skipped %d zero-byte file(s).\n", len(zeroByteFiles))
	}
	return nil
}

func scopedTask(mode string, task config.Task, selected string) (config.Task, error) {
	root, err := filepath.Abs(config.ToNativePath(task.Local))
	if err != nil {
		return task, err
	}
	path, err := filepath.Abs(config.ToNativePath(strings.TrimSpace(selected)))
	if err != nil {
		return task, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return task, err
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return task, err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return task, fmt.Errorf("%s is outside task %s local path %s", path, task.Name, root)
	}
	if mode == "sync" && !info.IsDir() {
		return task, fmt.Errorf("--path file is only supported for update")
	}

	task.Local = path
	if rel == "." {
		return task, nil
	}
	remoteRel := filepath.ToSlash(rel)
	if !info.IsDir() {
		remoteRel = filepath.ToSlash(filepath.Dir(rel))
		if remoteRel == "." {
			remoteRel = ""
		}
	}
	if remoteRel != "" {
		task.Remote = config.TrimRemotePath(task.Remote) + "/" + remoteRel
	}
	return task, nil
}

func (r Runner) runOutput(name string, args ...string) (string, error) {
	if r.output != nil {
		return r.output(name, args...)
	}
	cmd := proc.CommandContext(r.context(), name, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
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
