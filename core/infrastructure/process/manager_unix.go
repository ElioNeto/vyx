//go:build !windows

package process

import (
	"os/exec"
	"syscall"
)

func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func stopProcess(cmd *exec.Cmd) error {
	return cmd.Process.Signal(syscall.SIGTERM)
}

func killProcess(cmd *exec.Cmd) error {
	return cmd.Process.Signal(syscall.SIGKILL)
}
