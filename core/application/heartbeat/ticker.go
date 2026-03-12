package heartbeat

import "time"

// ticker wraps time.Ticker so that tests can substitute a fake.
// Production code uses newTicker which returns a real *time.Ticker.
type ticker interface {
	Stop()
	Chan() <-chan time.Time
}

type realTicker struct{ t *time.Ticker }

func (r *realTicker) Stop()                  { r.t.Stop() }
func (r *realTicker) Chan() <-chan time.Time  { return r.t.C }

func newTicker(d time.Duration) ticker {
	return &realTicker{t: time.NewTicker(d)}
}
