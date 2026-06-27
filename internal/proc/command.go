package proc

import "os/exec"

func Command(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	configureHidden(cmd)
	return cmd
}

func Detach(cmd *exec.Cmd) {
	configureDetached(cmd)
}
