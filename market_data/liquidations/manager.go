// Пакет liquidations — управление данными о ликвидациях.
// manager.go — накопление ликвидаций с TTL 60 сек.
package liquidations

import (
	"sync"
	"time"

	"bybit-elite-signal/api"
	"go.uber.org/zap"
)

const ttl = 60 * time.Second

type entry struct {
	data      []api.LiquidationRecord
	fetchedAt time.Time
}

// Cluster — кластер ликвидаций в ценовой зоне.
type Cluster struct {
	Price    float64
	Side     string
	Notional float64
}

// Manager накапливает записи ликвидаций с TTL кешем.
type Manager struct {
	client *api.Client
	log    *zap.Logger
	mu     sync.RWMutex
	data   map[string]entry
	maxLen int
}

// NewManager создаёт Manager.
func NewManager(client *api.Client, log *zap.Logger) *Manager {
	return &Manager{client: client, log: log, data: make(map[string]entry), maxLen: 1000}
}

// Fetch загружает ликвидации только если TTL истёк. Возвращает true если был REST вызов.
func (m *Manager) Fetch(symbol string) bool {
	if !m.isStale(symbol) {
		return false
	}
	liq, err := m.client.GetLiquidations(symbol, 200)
	if err != nil {
		m.log.Warn("ошибка ликвидаций", zap.String("symbol", symbol), zap.Error(err))
		return false
	}
	m.mu.Lock()
	combined := append(liq, m.data[symbol].data...)
	if len(combined) > m.maxLen {
		combined = combined[:m.maxLen]
	}
	m.data[symbol] = entry{data: combined, fetchedAt: time.Now()}
	m.mu.Unlock()
	return true
}

// Get возвращает список ликвидаций.
func (m *Manager) Get(symbol string) []api.LiquidationRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.data[symbol].data
}

// Clusters возвращает ценовые кластеры принудительных ликвидаций.
func (m *Manager) Clusters(symbol string, currentPrice float64) []Cluster {
	records := m.Get(symbol)
	if currentPrice <= 0 {
		return nil
	}
	buckets := make(map[int]float64)
	for _, r := range records {
		p, _ := r.Price.Float64()
		q, _ := r.Qty.Float64()
		buckets[int(p/(currentPrice*0.01))] += p * q
	}
	clusters := make([]Cluster, 0)
	for bucket, notional := range buckets {
		if notional < 50_000 {
			continue
		}
		price := float64(bucket) * currentPrice * 0.01
		side := "Long"
		if price > currentPrice {
			side = "Short"
		}
		clusters = append(clusters, Cluster{Price: price, Side: side, Notional: notional})
	}
	return clusters
}

// IsStale — публичная версия для воркера.
func (m *Manager) IsStale(symbol string) bool { return m.isStale(symbol) }

func (m *Manager) isStale(symbol string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.data[symbol]
	return !ok || time.Since(e.fetchedAt) >= ttl
}
