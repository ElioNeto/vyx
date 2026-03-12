//go:build windows

package process

import (
	"os/exec"
	"syscall"
)

func setProcAttr(cmd *exec.Cmd) {
	// On Windows, Setpgid does not exist. CREATE_NEW_PROCESS_GROUP is the
	// equivalent: it isolates the child so Ctrl+C is not propagated.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

func stopProcess(cmd *exec.Cmd) error {
	// Windows has no SIGTERM; Kill is the only reliable termination signal.
	return cmd.Process.Kill()
}

func killProcess(cmd *exec.Cmd) error {
	return cmd.Process.Kill()
}
