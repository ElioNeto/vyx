//go:build windows

package process_test

// ping -n 31 127.0.0.1 waits ~30 seconds (1 reply per second, 31 iterations).
const sleepCmd = "ping"

var sleepArgs = []string{"-n", "31", "127.0.0.1"}
