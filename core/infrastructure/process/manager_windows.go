//go:build windows

package process

import (
	"os/exec"
	"strings"
	"syscall"
)

func setProcAttr(cmd *exec.Cmd) {
	// On Windows, Setpgid does not exist. CREATE_NEW_PROCESS_GROUP is the
	// equivalent: it isolates the child so Ctrl+C is not propagated.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// isAccessDenied returns true for the Windows "Access is denied" error that
// TerminateProcess returns when the target process has already exited.
func isAccessDenied(err error) bool {
	return err != nil && strings.Contains(err.Error(), "Access is denied")
}

func stopProcess(cmd *exec.Cmd) error {
	// Windows has no SIGTERM; Kill is the only reliable termination signal.
	if err := cmd.Process.Kill(); err != nil && !isAccessDenied(err) {
		return err
	}
	return nil
}

func killProcess(cmd *exec.Cmd) error {
	if err := cmd.Process.Kill(); err != nil && !isAccessDenied(err) {
		return err
	}
	return nil
}
