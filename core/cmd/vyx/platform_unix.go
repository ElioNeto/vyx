//go:build !windows

package main

// isWindows returns false on non-Windows builds.
func isWindows() bool { return false }
