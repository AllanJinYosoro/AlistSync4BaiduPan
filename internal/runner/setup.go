package runner

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"bdp-sync/internal/config"
	"bdp-sync/internal/deps"
)

func (r Runner) cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(r.stderr)
	configPath := fs.String("config", config.DefaultPath, "config file path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := config.EnsureLocalDirs(); err != nil {
		return err
	}
	created, err := config.WriteSampleIfMissing(*configPath)
	if err != nil {
		return err
	}
	if created {
		fmt.Fprintln(r.stdout, "created", *configPath)
	} else {
		fmt.Fprintln(r.stdout, "kept existing", *configPath)
	}
	if err := ensureGitignore(); err != nil {
		return err
	}
	fmt.Fprintln(r.stdout, "created local state directory", config.LocalStateDir)
	return nil
}

func (r Runner) cmdSetup(args []string) error {
	if len(args) == 0 {
		return errors.New("setup requires subcommand: deps")
	}
	switch args[0] {
	case "deps":
		return r.cmdSetupDeps(args[1:])
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
	return deps.EnsureAll(*force, r.stdout)
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
