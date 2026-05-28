//go:build windows

package process

import (
	"os/exec"
	"strings"
	"syscall"
)

var (
	modkernel32                  = syscall.NewLazyDLL("kernel32.dll")
	procGenerateConsoleCtrlEvent = modkernel32.NewProc("GenerateConsoleCtrlEvent")
)

const (
	// CTRL_BREAK_EVENT is the control code for GenerateConsoleCtrlEvent.
	// Unlike CTRL_C_EVENT, it is always delivered to the target process group
	// regardless of whether the calling process is a debugger or has a console.
	CTRL_BREAK_EVENT = 1
)

// generateConsoleCtrlEvent sends a control signal to a process group.
// It is the Windows equivalent of sending SIGTERM to a process group on Unix.
func generateConsoleCtrlEvent(ctrlEvent int, processGroupID uint32) error {
	r, _, err := procGenerateConsoleCtrlEvent.Call(uintptr(ctrlEvent), uintptr(processGroupID))
	if r == 0 {
		return err
	}
	return nil
}

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

// stopProcess sends CTRL_BREAK_EVENT to the process group, giving the worker
// a chance to shut down gracefully (analogous to SIGTERM on Unix).
// The caller (Manager.Stop) waits for the process to exit and falls back to
// killProcess if the shutdown times out.
func stopProcess(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	_ = generateConsoleCtrlEvent(CTRL_BREAK_EVENT, uint32(cmd.Process.Pid))
	return nil
}

// killProcess forcefully terminates the process via TerminateProcess
// (cmd.Process.Kill on Windows). This is the fallback when the process
// does not exit after the graceful shutdown timeout.
func killProcess(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	if err := cmd.Process.Kill(); err != nil && !isKillError(err) {
		return err
	}
	return nil
}
