package config

import domaincfg "github.com/ElioNeto/vyx/core/domain/config"

// SetCurrent sets the current config directly. Used in tests and at startup
// after the initial Load() to seed the in-memory cache.
func (l *Loader) SetCurrent(cfg *domaincfg.Config) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.current = cfg
}
