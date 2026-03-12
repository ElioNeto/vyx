package gateway

import (
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

func (b *bucket) allow(limit int, window time.Duration) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	if now.After(b.windowEnd) {
		b.count = 0
		b.windowEnd = now.Add(window)
	}
	if b.count >= limit {
		return false
	}
	b.count++
	return true
}

// RateLimiter enforces per-IP and per-token request limits using fixed windows.
type RateLimiter struct {
	perIP    int
	perToken int
	window   time.Duration

	ipMu     sync.RWMutex
	ipBuckets map[string]*bucket

	tokMu      sync.RWMutex
	tokBuckets map[string]*bucket
}

// NewRateLimiter creates a RateLimiter with the given per-IP and per-token limits.
// window is the rolling time window (typically 1 minute).
func NewRateLimiter(perIP, perToken int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		perIP:      perIP,
		perToken:   perToken,
		window:     window,
		ipBuckets:  make(map[string]*bucket),
		tokBuckets: make(map[string]*bucket),
	}
}

// AllowIP returns true if the given remote address is within the per-IP limit.
func (r *RateLimiter) AllowIP(remoteAddr string) bool {
	ip, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		ip = remoteAddr
	}
	return r.getBucket(&r.ipMu, r.ipBuckets, ip).allow(r.perIP, r.window)
}

// AllowToken returns true if the given token is within the per-token limit.
func (r *RateLimiter) AllowToken(token string) bool {
	if token == "" {
		return true
	}
	return r.getBucket(&r.tokMu, r.tokBuckets, token).allow(r.perToken, r.window)
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
