package finnhub_limiter

import (
	"sync"
	"time"
)

// Limiter controls the rate of API requests.
type Limiter struct {
	lastRequest time.Time
	interval    time.Duration
	mu          sync.Mutex
}

// NewLimiter creates a new Limiter with the specified interval between requests.
func NewLimiter(interval time.Duration) *Limiter {
	return &Limiter{
		interval: interval,
	}
}

// Wait waits until it's safe to make another request.
func (l *Limiter) Wait() {
	l.mu.Lock()
	defer l.mu.Unlock()

	elapsed := time.Since(l.lastRequest)
	if elapsed < l.interval {
		time.Sleep(l.interval - elapsed)
	}
	l.lastRequest = time.Now()
}
