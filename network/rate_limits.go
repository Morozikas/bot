// Пакет network — работа с сетью.
// rate_limits.go — token-bucket лимитер с очередью запросов.
// Bybit public API: 120 req/min на IP. Держим 80 req/min (запас 33%).
package network

import (
	"sync"
	"time"
)

// RateLimiter — классический token-bucket.
type RateLimiter struct {
	mu       sync.Mutex
	tokens   float64
	capacity float64
	refillPS float64 // токенов в секунду
	lastTime time.Time
}

// NewRateLimiter создаёт лимитер.
func NewRateLimiter(capacity, refillPerSecond float64) *RateLimiter {
	return &RateLimiter{
		tokens:   capacity,
		capacity: capacity,
		refillPS: refillPerSecond,
		lastTime: time.Now(),
	}
}

// Allow возвращает true если запрос разрешён.
func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	r.tokens += now.Sub(r.lastTime).Seconds() * r.refillPS
	r.lastTime = now
	if r.tokens > r.capacity {
		r.tokens = r.capacity
	}
	if r.tokens >= 1 {
		r.tokens--
		return true
	}
	return false
}

// Wait блокируется пока не получит токен.
func (r *RateLimiter) Wait() {
	for !r.Allow() {
		time.Sleep(20 * time.Millisecond)
	}
}

// WaitTimeout пытается получить токен в течение timeout.
// Возвращает false если время вышло.
func (r *RateLimiter) WaitTimeout(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if r.Allow() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// BybitPublicLimiter — лимитер под публичный REST API Bybit.
// Используем 80 req/min вместо 120 — запас 33% на случай временных пиков.
// Burst = 5 токенов (короткие всплески разрешены).
func BybitPublicLimiter() *RateLimiter {
	return NewRateLimiter(5, 80.0/60.0) // ~1.33 req/sec sustained, burst 5
}

// BybitPrivateLimiter — лимитер для приватных эндпоинтов (ордера, позиции).
// Приватные лимиты строже: 10 req/sec per endpoint, но мы торгуем редко.
func BybitPrivateLimiter() *RateLimiter {
	return NewRateLimiter(3, 5.0/60.0) // 5 req/min на торговые операции
}

// Throttle — последовательный регулятор для пакетных запросов.
// Вместо параллельной стрельбы N запросов одновременно — равномерно по времени.
type Throttle struct {
	limiter  *RateLimiter
	minDelay time.Duration // минимальная задержка между вызовами
}

// NewThrottle создаёт Throttle.
// minDelayMs: минимальный интервал между запросами в мс.
func NewThrottle(limiter *RateLimiter, minDelayMs int) *Throttle {
	return &Throttle{
		limiter:  limiter,
		minDelay: time.Duration(minDelayMs) * time.Millisecond,
	}
}

// Do ждёт разрешения лимитера + минимальную задержку, затем вызывает fn.
func (t *Throttle) Do(fn func()) {
	t.limiter.Wait()
	if t.minDelay > 0 {
		time.Sleep(t.minDelay)
	}
	fn()
}
