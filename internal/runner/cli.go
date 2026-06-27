package runner

import (
	"fmt"
	"io"
	"os/exec"
)

type Runner struct {
	stdout io.Writer
	stderr io.Writer
	exec   func(name string, args ...string) error
	start  func(name string, args ...string) error
	output func(name string, args ...string) (string, error)
}

func New(stdout, stderr io.Writer) Runner {
	return Runner{
		stdout: stdout,
		stderr: stderr,
		exec: func(name string, args ...string) error {
			cmd := exec.Command(name, args...)
			cmd.Stdout = stdout
			cmd.Stderr = stderr
			return cmd.Run()
		},
		start: func(name string, args ...string) error {
			cmd := exec.Command(name, args...)
			cmd.Stdout = stdout
			cmd.Stderr = stderr
			if err := cmd.Start(); err != nil {
				return err
			}
			return cmd.Process.Release()
		},
		output: func(name string, args ...string) (string, error) {
			cmd := exec.Command(name, args...)
			cmd.Stderr = stderr
			out, err := cmd.Output()
			return string(out), err
		},
	}
}

func (r Runner) Run(args []string) error {
	if len(args) == 0 {
		printUsage(r.stdout)
		return nil
	}

	switch args[0] {
	case "init":
		return r.cmdInit(args[1:])
	case "setup":
		return r.cmdSetup(args[1:])
	case "doctor":
		return r.cmdDoctor(args[1:])
	case "dry-run", "sync", "update":
		return r.cmdTransfer(args[0], args[1:])
	case "help", "-h", "--help":
		printUsage(r.stdout)
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `bdp-sync - AList + rclone backup sync for Baidu Netdisk

Usage:
  bdp-sync init
  bdp-sync setup deps
  bdp-sync doctor
  bdp-sync dry-run [task|--all]
  bdp-sync update [task|--all]
  bdp-sync sync [task|--all]

Global flags:
  --config PATH     Config file path, defaults to config.yaml
`)
}
