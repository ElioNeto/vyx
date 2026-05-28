package recovery

import (
	"go.uber.org/zap"
)

// ZapAdapter adapts a *zap.Logger to the recovery.Logger interface.
//
// Usage:
//
//	adapter := &recovery.ZapAdapter{Logger: log}
//	defer recovery.LogPanic(adapter, "my_component", nil)
type ZapAdapter struct {
	*zap.Logger
}

// Error logs a structured error message via zap.
func (a *ZapAdapter) Error(msg string, keysAndValues ...interface{}) {
	a.Logger.Error(msg, toZapFields(keysAndValues)...)
}

// Warn logs a structured warning message via zap.
func (a *ZapAdapter) Warn(msg string, keysAndValues ...interface{}) {
	a.Logger.Warn(msg, toZapFields(keysAndValues)...)
}

// toZapFields converts a key-value slice (as used by recovery.Logger) to
// a []zap.Field slice suitable for *zap.Logger.
func toZapFields(kv []interface{}) []zap.Field {
	if len(kv) == 0 {
		return nil
	}
	fields := make([]zap.Field, 0, len(kv)/2+1)
	for i := 0; i < len(kv)-1; i += 2 {
		key, ok := kv[i].(string)
		if !ok {
			continue
		}
		fields = append(fields, zap.Any(key, kv[i+1]))
	}
	// If odd number of elements, append the last one as "extra"
	if len(kv)%2 != 0 {
		fields = append(fields, zap.Any("extra", kv[len(kv)-1]))
	}
	return fields
}
