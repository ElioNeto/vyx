//go:build windows

package process

import (
	"os/exec"
	"strings"
	"syscall"
)

func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// isKillError returns true for Windows errors that mean the process is already
// gone: "Access is denied" (TerminateProcess on exited process) and
// "invalid argument" (handle already closed by a previous Wait/Kill call).
func isKillError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "Access is denied") ||
		strings.Contains(s, "invalid argument")
}

func stopProcess(cmd *exec.Cmd) error {
	if err := cmd.Process.Kill(); err != nil && !isKillError(err) {
		return err
	}
	return nil
}

func killProcess(cmd *exec.Cmd) error {
	if err := cmd.Process.Kill(); err != nil && !isKillError(err) {
		return err
	}
	return nil
}
