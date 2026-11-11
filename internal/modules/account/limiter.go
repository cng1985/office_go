package account

import (
	"sync"
	"time"
)

// Limiter 是登录和注册的进程内限流。键的数量有上限。
type Limiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	maxKeys int
	hits    map[string][]time.Time
}

func NewLimiter(limit int, window time.Duration, maxKeys int) *Limiter {
	return &Limiter{
		limit:   limit,
		window:  window,
		maxKeys: maxKeys,
		hits:    map[string][]time.Time{},
	}
}

func (l *Limiter) Allow(key string) bool {
	now := time.Now()
	cutoff := now.Add(-l.window)
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.hits[key]; !ok && len(l.hits) >= l.maxKeys {
		l.evict(cutoff)
		if _, ok := l.hits[key]; !ok && len(l.hits) >= l.maxKeys {
			return false
		}
	}
	kept := l.hits[key][:0]
	for _, hit := range l.hits[key] {
		if hit.After(cutoff) {
			kept = append(kept, hit)
		}
	}
	if len(kept) >= l.limit {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	return true
}

func (l *Limiter) evict(cutoff time.Time) {
	for key, hits := range l.hits {
		kept := hits[:0]
		for _, hit := range hits {
			if hit.After(cutoff) {
				kept = append(kept, hit)
			}
		}
		if len(kept) == 0 {
			delete(l.hits, key)
			continue
		}
		l.hits[key] = kept
	}
}
