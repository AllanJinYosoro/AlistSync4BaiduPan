//go:build !windows

package proc

import "os/exec"

func configureHidden(cmd *exec.Cmd) {}

func configureDetached(cmd *exec.Cmd) {}
