// Package metrics defines the instrumentation interfaces for the vyx core.
// Implementations (Prometheus, OpenTelemetry, etc.) live in infrastructure/metrics.
package metrics

import "time"

// Labels is a set of key-value pairs attached to metric observations.
type Labels map[string]string

// Counter is a monotonically increasing counter.
type Counter interface {
	Inc(labels Labels)
	Add(value float64, labels Labels)
}

// Gauge is a value that can go up and down.
type Gauge interface {
	Set(value float64, labels Labels)
	Inc(labels Labels)
	Dec(labels Labels)
	Add(value float64, labels Labels)
	Sub(value float64, labels Labels)
}

// Histogram observes value distributions.
type Histogram interface {
	Observe(value float64, labels Labels)
}

// Provider is the central metrics factory.
// A nil-safe implementation is available via Noop().
type Provider interface {
	// NewCounter creates or returns a registered counter.
	NewCounter(name, help string) Counter
	// NewGauge creates or returns a registered gauge.
	NewGauge(name, help string) Gauge
	// NewHistogram creates or returns a registered histogram with default buckets.
	NewHistogram(name, help string) Histogram
	// NewHistogramWithBuckets creates a histogram with custom bucket boundaries.
	NewHistogramWithBuckets(name, help string, buckets []float64) Histogram
	// HTTPHandler returns an http.Handler that exposes metrics (e.g. /metrics).
	HTTPHandler() interface{} // interface{} to avoid importing net/http at domain level
}

// HTTPHandler is a helper type alias so the domain can reference it.
// Infrastructure code will assert this to http.Handler.

// ---- noop implementation ----

type noopCounter struct{}
type noopGauge struct{}
type noopHistogram struct{}

func (noopCounter) Inc(Labels)              {}
func (noopCounter) Add(float64, Labels)     {}
func (noopGauge) Set(float64, Labels)       {}
func (noopGauge) Inc(Labels)                {}
func (noopGauge) Dec(Labels)                {}
func (noopGauge) Add(float64, Labels)       {}
func (noopGauge) Sub(float64, Labels)       {}
func (noopHistogram) Observe(float64, Labels) {}

type noopProvider struct{}

func (noopProvider) NewCounter(name, help string) Counter                                      { return noopCounter{} }
func (noopProvider) NewGauge(name, help string) Gauge                                          { return noopGauge{} }
func (noopProvider) NewHistogram(name, help string) Histogram                                   { return noopHistogram{} }
func (noopProvider) NewHistogramWithBuckets(name, help string, buckets []float64) Histogram      { return noopHistogram{} }
func (noopProvider) HTTPHandler() interface{}                                                    { return nil }

// Noop returns a Provider that silently discards all metrics.
// Use as a default when no monitoring is configured.
func Noop() Provider { return noopProvider{} }

// ---- Standard metric names ----

const (
	HTTPRequestTotal    = "vyx_http_requests_total"
	HTTPRequestDuration = "vyx_http_request_duration_seconds"
	HTTPRequestInFlight = "vyx_http_requests_in_flight"

	WorkerRequestTotal    = "vyx_worker_requests_total"
	WorkerRequestDuration = "vyx_worker_request_duration_seconds"
	WorkerErrorsTotal     = "vyx_worker_errors_total"

	PoolSize        = "vyx_worker_pool_size"
	PoolActive      = "vyx_worker_pool_active"
	PoolIdle        = "vyx_worker_pool_idle"
	PoolReplenished = "vyx_worker_pool_replenished_total"

	CircuitBreakerState = "vyx_circuit_breaker_state"
	CircuitBreakerTrips = "vyx_circuit_breaker_trips_total"
)

// Common label keys.
const (
	LabelMethod   = "method"
	LabelPath     = "path"
	LabelStatus   = "status"
	LabelWorker   = "worker"
	LabelSource   = "source"
	LabelState    = "state" // circuit breaker state
	LabelPhase    = "phase" // error phase
	LabelError    = "error"
)

// HTTP request phases for observability.
const (
	PhaseRouteMatch  = "route_match"
	PhaseAuth        = "auth"
	PhaseValidation  = "validation"
	PhaseRateLimit   = "rate_limit"
	PhaseDispatch    = "dispatch"
	PhaseResponse    = "response"
)

// DurationSince is a convenience helper for histogram observations.
func DurationSince(start time.Time, h Histogram, labels Labels) {
	h.Observe(time.Since(start).Seconds(), labels)
}
