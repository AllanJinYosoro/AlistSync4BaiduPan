package main

import (
	"fmt"
	"os"
	"os/exec"
)

func main() {
	r := Runner{
		stdout: os.Stdout,
		stderr: os.Stderr,
		exec: func(name string, args ...string) error {
			cmd := exec.Command(name, args...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		},
		start:  startBackgroundProcess,
		output: runOutput,
	}
	if err := r.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
