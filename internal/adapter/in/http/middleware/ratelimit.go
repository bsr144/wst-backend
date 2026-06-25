package middleware

import (
	"context"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/time/rate"

	"wst-backend/internal/adapter/in/http/presenter"
	"wst-backend/internal/pkg/apperr"
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type IPRateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     rate.Limit
	burst    int
	ttl      time.Duration
}

func NewIPRateLimiter(rpm, burst int) *IPRateLimiter {
	return &IPRateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rate.Limit(float64(rpm) / 60.0),
		burst:    burst,
		ttl:      10 * time.Minute,
	}
}

func (l *IPRateLimiter) limiterFor(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	v, ok := l.visitors[ip]
	if !ok {
		v = &visitor{limiter: rate.NewLimiter(l.rate, l.burst)}
		l.visitors[ip] = v
	}
	v.lastSeen = time.Now()
	return v.limiter
}

func (l *IPRateLimiter) Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !l.limiterFor(c.IP()).Allow() {
			return presenter.Error(c, apperr.RateLimited(apperr.CodeRateLimited, apperr.MessageFor(apperr.CodeRateLimited)))
		}
		return c.Next()
	}
}

func (l *IPRateLimiter) StartJanitor(ctx context.Context) {
	ticker := time.NewTicker(l.ttl)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			l.evict()
		}
	}
}

func (l *IPRateLimiter) evict() {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := time.Now().Add(-l.ttl)
	for ip, v := range l.visitors {
		if v.lastSeen.Before(cutoff) {
			delete(l.visitors, ip)
		}
	}
}
