package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

func main() {
	if len(os.Args) == 1 {
		runGUI()
		return
	}

	r := NewRunner(os.Stdout, os.Stderr)
	if err := r.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func NewRunner(stdout, stderr io.Writer) Runner {
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
