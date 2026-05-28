package gateway

import (
	"math/rand" // nosemgrep: go.lang.security.audit.crypto.math_random.math-random-used
	"net"
	"sync"
	"time"
)

// bucket is a sliding-window counter for one key.
type bucket struct {
	mu        sync.Mutex
	count     int
	windowEnd time.Time
}

func (b *bucket) allow(limit int, window, jitter time.Duration) (bool, time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	if now.After(b.windowEnd) {
		b.count = 0
		b.windowEnd = now.Add(window)
		if jitter > 0 {
			b.windowEnd = b.windowEnd.Add(time.Duration(rand.Int63n(int64(jitter))))
		}
	}
	if b.count >= limit {
		retryAfter := b.windowEnd.Sub(now)
		if retryAfter < 0 {
			retryAfter = 0
		}
		return false, retryAfter
	}
	b.count++
	return true, 0
}

// RateLimiter enforces per-IP and per-token request limits.
//
// The default mode uses fixed windows that can exhibit a stampede at window
// boundaries: all blocked clients retry simultaneously when their window resets.
// To mitigate this, use NewRateLimiterWithJitter — it adds random noise to the
// window end time, spreading retries across the jitter interval.
//
// Example: with a 1-minute window and 5s jitter, each key's window end is
// offset by a random 0-5s, smoothing out the thundering herd effect.
type RateLimiter struct {
	perIP    int
	perToken int
	window   time.Duration
	jitter   time.Duration

	ipMu     sync.RWMutex
	ipBuckets map[string]*bucket

	tokMu      sync.RWMutex
	tokBuckets map[string]*bucket
}

// NewRateLimiter creates a RateLimiter with the given per-IP and per-token limits.
// window is the rolling time window (typically 1 minute).
// No jitter is applied — use NewRateLimiterWithJitter to mitigate window-boundary stampedes.
func NewRateLimiter(perIP, perToken int, window time.Duration) *RateLimiter {
	return NewRateLimiterWithJitter(perIP, perToken, window, 0)
}

// NewRateLimiterWithJitter creates a RateLimiter that adds random jitter to window
// reset times. This prevents the thundering herd problem where all blocked clients
// retry at exactly the same moment the window expires.
//
// The jitter is a random duration between 0 and maxJitter added to the window end
// time on each reset. A good starting value is 10-20% of the window duration.
// jitter is capped at window/2 to avoid unbounded drift.
func NewRateLimiterWithJitter(perIP, perToken int, window, maxJitter time.Duration) *RateLimiter {
	if maxJitter > window/2 {
		maxJitter = window / 2
	}
	return &RateLimiter{
		perIP:      perIP,
		perToken:   perToken,
		window:     window,
		jitter:     maxJitter,
		ipBuckets:  make(map[string]*bucket),
		tokBuckets: make(map[string]*bucket),
	}
}

// AllowIP returns true if the given remote address is within the per-IP limit.
// When denied, retryAfter is the duration the client should wait before retrying.
func (r *RateLimiter) AllowIP(remoteAddr string) (ok bool, retryAfter time.Duration) {
	ip, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		ip = remoteAddr
	}
	return r.getBucket(&r.ipMu, r.ipBuckets, ip).allow(r.perIP, r.window, r.jitter)
}

// AllowToken returns true if the given token is within the per-token limit.
// Empty tokens are always allowed. When denied, retryAfter is the duration
// the client should wait before retrying.
func (r *RateLimiter) AllowToken(token string) (ok bool, retryAfter time.Duration) {
	if token == "" {
		return true, 0
	}
	return r.getBucket(&r.tokMu, r.tokBuckets, token).allow(r.perToken, r.window, r.jitter)
}

// Jitter returns the configured jitter duration (0 if none).
func (r *RateLimiter) Jitter() time.Duration {
	return r.jitter
}

func (r *RateLimiter) getBucket(mu *sync.RWMutex, m map[string]*bucket, key string) *bucket {
	mu.RLock()
	b, ok := m[key]
	mu.RUnlock()
	if ok {
		return b
	}
	mu.Lock()
	defer mu.Unlock()
	if b, ok = m[key]; ok {
		return b
	}
	b = &bucket{}
	m[key] = b
	return b
}
