package runner

import (
	"context"
	"fmt"
	"io"

	"bdp-sync/internal/proc"
)

type Runner struct {
	ctx          context.Context
	stdout       io.Writer
	stderr       io.Writer
	exec         func(name string, args ...string) error
	start        func(name string, args ...string) error
	startManaged func(name string, args ...string) (func() error, error)
	output       func(name string, args ...string) (string, error)
}

func New(stdout, stderr io.Writer) Runner {
	return NewContext(context.Background(), stdout, stderr)
}

func NewContext(ctx context.Context, stdout, stderr io.Writer) Runner {
	if ctx == nil {
		ctx = context.Background()
	}
	return Runner{
		ctx:    ctx,
		stdout: stdout,
		stderr: stderr,
		exec: func(name string, args ...string) error {
			cmd := proc.CommandContext(ctx, name, args...)
			cmd.Stdout = stdout
			cmd.Stderr = stderr
			return cmd.Run()
		},
		start: func(name string, args ...string) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			cmd := proc.Command(name, args...)
			proc.Detach(cmd)
			cmd.Stdout = stdout
			cmd.Stderr = stderr
			if err := cmd.Start(); err != nil {
				return err
			}
			return cmd.Process.Release()
		},
		startManaged: nil,
		output: func(name string, args ...string) (string, error) {
			cmd := proc.CommandContext(ctx, name, args...)
			cmd.Stderr = stderr
			out, err := cmd.Output()
			return string(out), err
		},
	}
}

func (r Runner) context() context.Context {
	if r.ctx != nil {
		return r.ctx
	}
	return context.Background()
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
