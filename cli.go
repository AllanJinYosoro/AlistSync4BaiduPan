package main

import (
	"fmt"
	"io"
)

type Runner struct {
	stdout io.Writer
	stderr io.Writer
	exec   func(name string, args ...string) error
	start  func(name string, args ...string) error
	output func(name string, args ...string) (string, error)
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
	fmt.Fprint(w, `alist-sync - AList + rclone backup sync for Baidu Netdisk

Usage:
  alist-sync init
  alist-sync setup deps
  alist-sync doctor
  alist-sync dry-run [task|--all]
  alist-sync update [task|--all]
  alist-sync sync [task|--all]

Global flags:
  --config PATH     Config file path, defaults to config.yaml
`)
}
