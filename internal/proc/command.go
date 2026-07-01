package proc

import (
	"context"
	"os/exec"
)

func Command(name string, args ...string) *exec.Cmd {
	return CommandContext(context.Background(), name, args...)
}

func CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	configureHidden(cmd)
	return cmd
}

func Detach(cmd *exec.Cmd) {
	configureDetached(cmd)
}
