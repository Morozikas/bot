// Пакет oi — управление данными открытого интереса.
// manager.go — кеш OI с TTL: история не перезагружается чаще чем раз в 60 секунд.
package oi

import (
	"sync"
	"time"

	"bybit-elite-signal/api"
	"go.uber.org/zap"
)

const ttl = 60 * time.Second // OI история меняется медленно

type entry struct {
	data      []api.OpenInterest
	fetchedAt time.Time
}

// Manager кеширует историю OI с TTL.
type Manager struct {
	client *api.Client
	log    *zap.Logger
	mu     sync.RWMutex
	data   map[string]entry
}

// NewManager создаёт Manager.
func NewManager(client *api.Client, log *zap.Logger) *Manager {
	return &Manager{client: client, log: log, data: make(map[string]entry)}
}

// Fetch загружает историю OI только если TTL истёк. Возвращает true если был REST вызов.
func (m *Manager) Fetch(symbol string, limit int) bool {
	if !m.isStale(symbol) {
		return false
	}
	history, err := m.client.GetOpenInterest(symbol, "1h", limit)
	if err != nil {
		m.log.Warn("ошибка OI", zap.String("symbol", symbol), zap.Error(err))
		return false
	}
	m.mu.Lock()
	m.data[symbol] = entry{data: history, fetchedAt: time.Now()}
	m.mu.Unlock()
	return true
}

// Get возвращает кешированную историю OI.
func (m *Manager) Get(symbol string) []api.OpenInterest {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.data[symbol].data
}

// Delta возвращает изменение OI (%) между последними двумя записями.
func (m *Manager) Delta(symbol string) float64 {
	history := m.Get(symbol)
	if len(history) < 2 {
		return 0
	}
	curr, _ := history[0].OpenInterest.Float64()
	prev, _ := history[1].OpenInterest.Float64()
	if prev == 0 {
		return 0
	}
	return (curr - prev) / prev * 100
}

// IsStale — публичная версия для воркера: проверяет нужна ли загрузка.
func (m *Manager) IsStale(symbol string) bool { return m.isStale(symbol) }

func (m *Manager) isStale(symbol string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.data[symbol]
	return !ok || time.Since(e.fetchedAt) >= ttl
}
