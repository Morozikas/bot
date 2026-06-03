// Пакет funding — управление данными ставки финансирования.
// manager.go — кеш funding rate с TTL 60 сек: история не перезагружается лишний раз.
package funding

import (
	"sync"
	"time"

	"bybit-elite-signal/api"
	"go.uber.org/zap"
)

const ttl = 60 * time.Second

type entry struct {
	data      []api.FundingRate
	fetchedAt time.Time
}

// Manager кеширует историю funding rate.
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

// Fetch загружает историю funding только если TTL истёк. Возвращает true если был REST вызов.
func (m *Manager) Fetch(symbol string, limit int) bool {
	if !m.isStale(symbol) {
		return false
	}
	rates, err := m.client.GetFundingHistory(symbol, limit)
	if err != nil {
		m.log.Warn("ошибка funding", zap.String("symbol", symbol), zap.Error(err))
		return false
	}
	m.mu.Lock()
	m.data[symbol] = entry{data: rates, fetchedAt: time.Now()}
	m.mu.Unlock()
	return true
}

// Get возвращает кешированную историю funding rate.
func (m *Manager) Get(symbol string) []api.FundingRate {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.data[symbol].data
}

// Current возвращает текущую ставку финансирования.
func (m *Manager) Current(symbol string) float64 {
	rates := m.Get(symbol)
	if len(rates) == 0 {
		return 0
	}
	r, _ := rates[0].FundingRate.Float64()
	return r
}

// IsStale — публичная версия для воркера.
func (m *Manager) IsStale(symbol string) bool { return m.isStale(symbol) }

func (m *Manager) isStale(symbol string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.data[symbol]
	return !ok || time.Since(e.fetchedAt) >= ttl
}
