//go:build windows

package proc

import (
	"os/exec"
	"syscall"
)

const (
	detachedProcess       = 0x00000008
	createNewProcessGroup = 0x00000200
)

func configureHidden(cmd *exec.Cmd) {
	ensureSysProcAttr(cmd).HideWindow = true
}

func configureDetached(cmd *exec.Cmd) {
	attr := ensureSysProcAttr(cmd)
	attr.HideWindow = true
	attr.CreationFlags |= detachedProcess | createNewProcessGroup
}

func ensureSysProcAttr(cmd *exec.Cmd) *syscall.SysProcAttr {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	return cmd.SysProcAttr
}
