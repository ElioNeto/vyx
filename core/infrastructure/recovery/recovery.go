// Package recovery provides panic recovery helpers for production goroutines.
// Every go func() in the codebase should defer recovery.LogPanic as early as
// possible in the goroutine body to prevent an unhandled panic from killing
// the entire process.
package recovery

import (
	"fmt"
	"runtime"
	"time"
)

// Logger is the minimal structured logger required by LogPanic.
// A nil-safe adapter for *zap.Logger is provided by ZapAdapter in this package.
type Logger interface {
	Error(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
}

// LogPanic recovers from a panic in a goroutine, logs the stack trace, and
// optionally launches a restart function.
//
// Use in goroutines as:
//
//	go func() {
//	    defer recovery.LogPanic(log, "my loop", nil)
//	    // ... code ...
//	}()
//
// The component name is a human-readable label such as "uds.read_pump" or
// "heartbeat.sender".
//
// If restartFn is non-nil it is invoked in a fresh goroutine so that a loop
// can be re-entered after a panic.  Most callers should pass nil — the
// operator or a higher-level supervisor will restart the process.
func LogPanic(log Logger, component string, restartFn func()) {
	if r := recover(); r != nil {
		stack := make([]byte, 4096)
		n := runtime.Stack(stack, false)

		if log != nil {
			log.Error("panic recovered",
				"component", component,
				"panic", fmt.Sprintf("%v", r),
				"stack", string(stack[:n]),
				"time", time.Now().UTC().Format(time.RFC3339Nano),
			)
		}

		if restartFn != nil {
			go restartFn()
		}
	}
}
