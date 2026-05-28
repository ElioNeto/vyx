//go:build !windows

package process

import (
	"os/exec"
	"syscall"
)

func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// stopProcess sends SIGTERM to the process group (negative PID) so that
// all descendant processes (e.g. sleep spawned by a shell) are also
// signalled. The Setpgid option ensures the child and its descendants
// share the same process group.
func stopProcess(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		return cmd.Process.Signal(syscall.SIGTERM)
	}
	// Negative PID targets the entire process group.
	return syscall.Kill(-pgid, syscall.SIGTERM)
}

// killProcess sends SIGKILL to the process group to ensure all
// descendants are terminated, preventing orphan processes from
// holding stdout/stderr pipes open.
func killProcess(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		return cmd.Process.Signal(syscall.SIGKILL)
	}
	return syscall.Kill(-pgid, syscall.SIGKILL)
}
