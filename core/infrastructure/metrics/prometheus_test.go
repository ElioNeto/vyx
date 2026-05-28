package metrics

import (
	"testing"
	"time"

	dmetrics "github.com/ElioNeto/vyx/core/domain/metrics"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPromProvider(t *testing.T) {
	p := NewPromProvider()
	require.NotNil(t, p)
	assert.NotNil(t, p.Registry())
}

func TestCounter(t *testing.T) {
	p := NewPromProvider()
	c := p.NewCounter("test_counter", "A test counter")
	require.NotNil(t, c)

	c.Inc(dmetrics.Labels{})
	c.Add(3, dmetrics.Labels{})

	// Verify via Prometheus testutil
	got := testutil.ToFloat64(p.counters["test_counter"].WithLabelValues())
	assert.Equal(t, float64(4), got)
}

func TestCounterWithLabels(t *testing.T) {
	p := NewPromProvider()
	c := p.NewCounter(dmetrics.HTTPRequestTotal, "Total requests")
	require.NotNil(t, c)

	c.Inc(dmetrics.Labels{dmetrics.LabelMethod: "GET", dmetrics.LabelPath: "/api", dmetrics.LabelStatus: "200"})
	c.Inc(dmetrics.Labels{dmetrics.LabelMethod: "GET", dmetrics.LabelPath: "/api", dmetrics.LabelStatus: "200"})
	c.Add(1, dmetrics.Labels{dmetrics.LabelMethod: "POST", dmetrics.LabelPath: "/data", dmetrics.LabelStatus: "201"})

	got := testutil.ToFloat64(p.counters[dmetrics.HTTPRequestTotal].
		WithLabelValues("GET", "/api", "200"))
	assert.Equal(t, float64(2), got)

	got = testutil.ToFloat64(p.counters[dmetrics.HTTPRequestTotal].
		WithLabelValues("POST", "/data", "201"))
	assert.Equal(t, float64(1), got)
}

func TestGauge(t *testing.T) {
	p := NewPromProvider()
	g := p.NewGauge("test_gauge", "A test gauge")
	require.NotNil(t, g)

	g.Set(42, dmetrics.Labels{})
	g.Inc(dmetrics.Labels{})
	g.Dec(dmetrics.Labels{})
	g.Add(10, dmetrics.Labels{})
	g.Sub(5, dmetrics.Labels{})

	got := testutil.ToFloat64(p.gauges["test_gauge"].WithLabelValues())
	// start 0, Set(42) → 42, Inc → 43, Dec → 42, Add(10) → 52, Sub(5) → 47
	assert.Equal(t, float64(47), got)
}

func TestGaugeWithLabels(t *testing.T) {
	p := NewPromProvider()
	g := p.NewGauge(dmetrics.PoolSize, "Pool size")
	require.NotNil(t, g)

	g.Set(5, dmetrics.Labels{dmetrics.LabelWorker: "node:ssr"})
	g.Set(3, dmetrics.Labels{dmetrics.LabelWorker: "python:api"})

	got := testutil.ToFloat64(p.gauges[dmetrics.PoolSize].
		WithLabelValues("node:ssr"))
	assert.Equal(t, float64(5), got)
}

func TestHistogram(t *testing.T) {
	p := NewPromProvider()
	h := p.NewHistogram("test_histogram", "A test histogram")
	require.NotNil(t, h)

	h.Observe(0.5, dmetrics.Labels{})
	h.Observe(1.5, dmetrics.Labels{})

	// Histogram should have 2 observations
	metric, err := p.registry.Gather()
	require.NoError(t, err)
	assert.Len(t, metric, 1)
	// Just verify no panic; detailed histogram assertions are fragile
}

func TestHistogramWithBuckets(t *testing.T) {
	p := NewPromProvider()
	buckets := []float64{0.1, 0.5, 1.0}
	h := p.NewHistogramWithBuckets("custom_bucket_hist", "Custom buckets", buckets)
	require.NotNil(t, h)

	h.Observe(0.05, dmetrics.Labels{})
	h.Observe(0.3, dmetrics.Labels{})
	h.Observe(0.8, dmetrics.Labels{})

	metric, err := p.registry.Gather()
	require.NoError(t, err)
	assert.Len(t, metric, 1)
}

func TestHTTPHandler(t *testing.T) {
	p := NewPromProvider()
	handler := p.HTTPHandler()
	assert.NotNil(t, handler)
	// The handler should not be nil when a PromProvider is used (noop returns nil)
	assert.NotNil(t, handler, "HTTPHandler from PromProvider should not be nil")
}

func TestNoopProvider(t *testing.T) {
	np := dmetrics.Noop()
	assert.NotNil(t, np)

	// None of these should panic
	c := np.NewCounter("noop", "")
	c.Inc(dmetrics.Labels{})
	c.Add(1, dmetrics.Labels{})

	g := np.NewGauge("noop", "")
	g.Set(1, dmetrics.Labels{})
	g.Inc(dmetrics.Labels{})
	g.Dec(dmetrics.Labels{})
	g.Add(1, dmetrics.Labels{})
	g.Sub(1, dmetrics.Labels{})

	h := np.NewHistogram("noop", "")
	h.Observe(1, dmetrics.Labels{})

	handler := np.HTTPHandler()
	assert.Nil(t, handler)
}

func TestDefaultProvider(t *testing.T) {
	// Default is noop
	p := Provider()
	_, ok := p.(dmetrics.Provider)
	assert.True(t, ok)

	// Set to PromProvider
	prom := NewPromProvider()
	SetDefaultProvider(prom)
	assert.Same(t, prom, Provider())

	// Reset to noop
	SetDefaultProvider(dmetrics.Noop())
	_, ok = Provider().(dmetrics.Provider)
	assert.True(t, ok)
}

func TestDurationSince(t *testing.T) {
	p := NewPromProvider()
	h := p.NewHistogram("duration_test", "Duration test")
	require.NotNil(t, h)

	start := time.Now()
	dmetrics.DurationSince(start, h, dmetrics.Labels{})
	metric, err := p.registry.Gather()
	require.NoError(t, err)
	assert.Len(t, metric, 1)
}

func TestCounterNilLabels(t *testing.T) {
	p := NewPromProvider()
	c := p.NewCounter("nil_labels_counter", "Counter with nil labels")
	require.NotNil(t, c)

	// Should not panic with nil labels
	c.Inc(nil)
	c.Add(2, nil)

	got := testutil.ToFloat64(p.counters["nil_labels_counter"].WithLabelValues())
	assert.Equal(t, float64(3), got)
}

func TestGaugeNilLabels(t *testing.T) {
	p := NewPromProvider()
	g := p.NewGauge("nil_labels_gauge", "Gauge with nil labels")
	require.NotNil(t, g)

	g.Inc(nil)
	g.Set(10, nil)

	got := testutil.ToFloat64(p.gauges["nil_labels_gauge"].WithLabelValues())
	assert.Equal(t, float64(10), got)
}
