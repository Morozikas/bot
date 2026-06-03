// Пакет orderbook — управление данными стакана заявок.
// manager.go — хранит актуальные снимки стакана, поддерживает delta-обновления.
package orderbook

import (
	"sync"

	"bybit-elite-signal/api"
	"go.uber.org/zap"
)

// Manager хранит актуальные стаканы по символам.
type Manager struct {
	client *api.Client
	log    *zap.Logger
	mu     sync.RWMutex
	data   map[string]*api.OrderBook
}

// NewManager создаёт Manager.
func NewManager(client *api.Client, log *zap.Logger) *Manager {
	return &Manager{client: client, log: log, data: make(map[string]*api.OrderBook)}
}

// Fetch загружает снимок стакана для символа.
func (m *Manager) Fetch(symbol string, depth int) {
	ob, err := m.client.GetOrderBook(symbol, depth)
	if err != nil {
		m.log.Warn("ошибка стакана", zap.String("symbol", symbol), zap.Error(err))
		return
	}
	m.mu.Lock()
	m.data[symbol] = ob
	m.mu.Unlock()
}

// Get возвращает актуальный стакан.
func (m *Manager) Get(symbol string) *api.OrderBook {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.data[symbol]
}

// BestBid возвращает лучшую цену покупки.
func (m *Manager) BestBid(symbol string) float64 {
	ob := m.Get(symbol)
	if ob == nil || len(ob.Bids) == 0 {
		return 0
	}
	p, _ := ob.Bids[0].Price.Float64()
	return p
}

// BestAsk возвращает лучшую цену продажи.
func (m *Manager) BestAsk(symbol string) float64 {
	ob := m.Get(symbol)
	if ob == nil || len(ob.Asks) == 0 {
		return 0
	}
	p, _ := ob.Asks[0].Price.Float64()
	return p
}

// Spread возвращает спред стакана в %.
func (m *Manager) Spread(symbol string) float64 {
	bid := m.BestBid(symbol)
	ask := m.BestAsk(symbol)
	if bid <= 0 || ask <= 0 {
		return 0
	}
	return (ask - bid) / ask * 100
}
