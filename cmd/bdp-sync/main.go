package main

import (
	"fmt"
	"os"

	"bdp-sync/internal/gui"
	"bdp-sync/internal/runner"
)

func main() {
	if len(os.Args) == 1 {
		gui.Run()
		return
	}

	r := runner.New(os.Stdout, os.Stderr)
	if err := r.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
