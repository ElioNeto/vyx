// Package metrics implements the domain metrics.Provider interface using Prometheus.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	dmetrics "github.com/ElioNeto/vyx/core/domain/metrics"
)

// PromProvider is a domain metrics.Provider backed by Prometheus.
type PromProvider struct {
	registry *prometheus.Registry

	counters   map[string]*prometheus.CounterVec
	gauges     map[string]*prometheus.GaugeVec
	histograms map[string]*prometheus.HistogramVec
}

// NewPromProvider creates a PromProvider with an isolated registry.
func NewPromProvider() *PromProvider {
	return &PromProvider{
		registry:   prometheus.NewRegistry(),
		counters:   make(map[string]*prometheus.CounterVec),
		gauges:     make(map[string]*prometheus.GaugeVec),
		histograms: make(map[string]*prometheus.HistogramVec),
	}
}

// Registry exposes the underlying prometheus registry for testing or custom registration.
func (p *PromProvider) Registry() *prometheus.Registry {
	if p == nil {
		return nil
	}
	return p.registry
}

// ---- label conversion ----

func labelKeys(l dmetrics.Labels) []string {
	keys := make([]string, 0, len(l))
	for k := range l {
		keys = append(keys, k)
	}
	return keys
}

func promLabels(l dmetrics.Labels) prometheus.Labels {
	out := make(prometheus.Labels, len(l))
	for k, v := range l {
		out[k] = v
	}
	return out
}

// ---- Counter ----

type promCounter struct {
	vec *prometheus.CounterVec
}

func (c *promCounter) Inc(l dmetrics.Labels) {
	if l == nil {
		c.vec.With(prometheus.Labels{}).Inc()
		return
	}
	c.vec.With(promLabels(l)).Inc()
}

func (c *promCounter) Add(v float64, l dmetrics.Labels) {
	if l == nil {
		c.vec.With(prometheus.Labels{}).Add(v)
		return
	}
	c.vec.With(promLabels(l)).Add(v)
}

// ---- Gauge ----

type promGauge struct {
	vec *prometheus.GaugeVec
}

func (g *promGauge) Set(v float64, l dmetrics.Labels) {
	g.vec.With(promLabels(l)).Set(v)
}

func (g *promGauge) Inc(l dmetrics.Labels) {
	g.vec.With(promLabels(l)).Inc()
}

func (g *promGauge) Dec(l dmetrics.Labels) {
	g.vec.With(promLabels(l)).Dec()
}

func (g *promGauge) Add(v float64, l dmetrics.Labels) {
	g.vec.With(promLabels(l)).Add(v)
}

func (g *promGauge) Sub(v float64, l dmetrics.Labels) {
	g.vec.With(promLabels(l)).Sub(v)
}

// ---- Histogram ----

type promHistogram struct {
	vec *prometheus.HistogramVec
}

func (h *promHistogram) Observe(v float64, l dmetrics.Labels) {
	h.vec.With(promLabels(l)).Observe(v)
}

// ---- Provider implementation ----

func (p *PromProvider) newCounterVec(name, help string, labels []string) *prometheus.CounterVec {
	cv := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: name,
		Help: help,
	}, labels)
	p.registry.MustRegister(cv)
	return cv
}

func (p *PromProvider) NewCounter(name, help string) dmetrics.Counter {
	if existing, ok := p.counters[name]; ok {
		return &promCounter{vec: existing}
	}
	// Start with no extra labels; use With(promLabels(l)) at observation time.
	// We pre-allocate the variable-label approach: the vec uses the label keys
	// from the first observation. But prometheus requires label names known at
	// registration time. So we don't use dynamic labels; instead we use
	// prometheus.Labels at record time. Users pass Labels - we convert them.
	// The counter vec is registered with empty variable labels and we use
	// With(promLabels(...)) to pass them. This works because prometheus.CounterVec
	// does NOT require pre-declaring label names when you use With() — actually
	// it DOES require them. So we need to know label names upfront.
	//
	// The simplest approach: register with a fixed set of empty labels based on
	// the standard metric conventions. Or we can be smarter and infer them from
	// the first call. Let's use a fixed set of common label names for each metric.
	var cv *prometheus.CounterVec
	switch name {
	case dmetrics.HTTPRequestTotal:
		cv = p.newCounterVec(name, help, []string{dmetrics.LabelMethod, dmetrics.LabelPath, dmetrics.LabelStatus})
	case dmetrics.WorkerRequestTotal:
		cv = p.newCounterVec(name, help, []string{dmetrics.LabelWorker, dmetrics.LabelMethod})
	case dmetrics.WorkerErrorsTotal:
		cv = p.newCounterVec(name, help, []string{dmetrics.LabelWorker, dmetrics.LabelPhase})
	case dmetrics.CircuitBreakerTrips:
		cv = p.newCounterVec(name, help, []string{dmetrics.LabelWorker})
	default:
		cv = p.newCounterVec(name, help, nil)
	}
	p.counters[name] = cv
	return &promCounter{vec: cv}
}

func (p *PromProvider) NewGauge(name, help string) dmetrics.Gauge {
	if existing, ok := p.gauges[name]; ok {
		return &promGauge{vec: existing}
	}
	var gv *prometheus.GaugeVec
	switch name {
	case dmetrics.HTTPRequestInFlight:
		gv = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: name, Help: help},
			[]string{dmetrics.LabelMethod, dmetrics.LabelPath})
	case dmetrics.PoolSize, dmetrics.PoolActive, dmetrics.PoolIdle:
		gv = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: name, Help: help},
			[]string{dmetrics.LabelWorker})
	case dmetrics.CircuitBreakerState:
		gv = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: name, Help: help},
			[]string{dmetrics.LabelWorker, dmetrics.LabelState})
	default:
		gv = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: name, Help: help}, nil)
	}
	p.registry.MustRegister(gv)
	p.gauges[name] = gv
	return &promGauge{vec: gv}
}

func (p *PromProvider) NewHistogram(name, help string) dmetrics.Histogram {
	return p.NewHistogramWithBuckets(name, help, prometheus.DefBuckets)
}

func (p *PromProvider) NewHistogramWithBuckets(name, help string, buckets []float64) dmetrics.Histogram {
	if existing, ok := p.histograms[name]; ok {
		return &promHistogram{vec: existing}
	}
	var hv *prometheus.HistogramVec
	switch name {
	case dmetrics.HTTPRequestDuration:
		hv = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: name, Help: help, Buckets: buckets},
			[]string{dmetrics.LabelMethod, dmetrics.LabelPath, dmetrics.LabelStatus})
	case dmetrics.WorkerRequestDuration:
		hv = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: name, Help: help, Buckets: buckets},
			[]string{dmetrics.LabelWorker, dmetrics.LabelMethod})
	default:
		hv = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: name, Help: help, Buckets: buckets}, nil)
	}
	p.registry.MustRegister(hv)
	p.histograms[name] = hv
	return &promHistogram{vec: hv}
}

// HTTPHandler returns an http.Handler that serves Prometheus metrics on /metrics.
func (p *PromProvider) HTTPHandler() interface{} {
	return promhttp.HandlerFor(p.registry, promhttp.HandlerOpts{})
}

// ensure interface compliance
var _ dmetrics.Provider = (*PromProvider)(nil)

// ---- Default global instance ----

var defaultProvider dmetrics.Provider

func init() {
	defaultProvider = dmetrics.Noop()
}

// SetDefaultProvider sets the global metrics provider.
func SetDefaultProvider(p dmetrics.Provider) {
	defaultProvider = p
}

// Provider returns the current default metrics provider.
func Provider() dmetrics.Provider {
	return defaultProvider
}
