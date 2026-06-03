// Пакет candles — хранение и обновление OHLCV-свечей по всем таймфреймам.
// manager.go — менеджер свечей с TTL-кешем: старшие ТФ не перезагружаются каждые 30 сек.
package candles

import (
	"sync"
	"time"

	"bybit-elite-signal/api"
	"go.uber.org/zap"
)

// ttlByTF — минимальный интервал между обновлениями по таймфрейму.
// Старшие ТФ меняются медленно — нет смысла их перечитывать при каждом скане.
var ttlByTF = map[string]time.Duration{
	"1":   60 * time.Second,  // 1M  — раз в минуту
	"5":   90 * time.Second,  // 5M  — раз в 1.5 мин
	"15":  3 * time.Minute,   // 15M — раз в 3 мин
	"60":  10 * time.Minute,  // 1H  — раз в 10 мин
	"240": 30 * time.Minute,  // 4H  — раз в 30 мин
	"D":   6 * time.Hour,     // 1D  — раз в 6 часов
}

// Timeframes — все таймфреймы для сбора.
var Timeframes = []string{"D", "240", "60", "15", "5"}

// entry — кешированные свечи с меткой времени последнего обновления.
type entry struct {
	candles   []api.Candle
	fetchedAt time.Time
}

// Manager хранит и обновляет свечи для всех символов с TTL-кешем.
type Manager struct {
	client *api.Client
	log    *zap.Logger
	mu     sync.RWMutex
	data   map[string]map[string]entry // symbol → tf → entry
	limit  int
}

// NewManager создаёт Manager.
func NewManager(client *api.Client, log *zap.Logger, limit int) *Manager {
	if limit <= 0 {
		limit = 300
	}
	return &Manager{
		client: client,
		log:    log,
		data:   make(map[string]map[string]entry),
		limit:  limit,
	}
}

// Fetch загружает свечи по всем таймфреймам, пропуская уже актуальные (TTL).
// Возвращает количество реально выполненных REST-запросов.
func (m *Manager) Fetch(symbol string) int {
	fetched := 0
	for _, tf := range Timeframes {
		if m.isStale(symbol, tf) {
			candles, err := m.client.GetCandles(symbol, tf, m.limit)
			if err != nil {
				m.log.Warn("ошибка загрузки свечей",
					zap.String("symbol", symbol),
					zap.String("tf", tf),
					zap.Error(err))
				continue
			}
			m.set(symbol, tf, candles)
			fetched++
		}
	}
	return fetched
}

// FetchFast загружает только быстрые таймфреймы (1M, 5M, 15M).
// Используется для символов, данные которых частично уже закешированы.
func (m *Manager) FetchFast(symbol string) int {
	fastTFs := []string{"1", "5", "15"}
	fetched := 0
	for _, tf := range fastTFs {
		if m.isStale(symbol, tf) {
			candles, err := m.client.GetCandles(symbol, tf, m.limit)
			if err != nil {
				m.log.Warn("ошибка загрузки fast candles",
					zap.String("symbol", symbol),
					zap.String("tf", tf),
					zap.Error(err))
				continue
			}
			m.set(symbol, tf, candles)
			fetched++
		}
	}
	return fetched
}

// Get возвращает кешированные свечи. Возвращает nil если данных нет.
func (m *Manager) Get(symbol, tf string) []api.Candle {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if sym, ok := m.data[symbol]; ok {
		if e, ok := sym[tf]; ok {
			return e.candles
		}
	}
	return nil
}

// IsFresh возвращает true если свечи для символа/ТФ ещё актуальны.
func (m *Manager) IsFresh(symbol, tf string) bool {
	return !m.isStale(symbol, tf)
}

// Last возвращает последнюю свечу.
func (m *Manager) Last(symbol, tf string) *api.Candle {
	candles := m.Get(symbol, tf)
	if len(candles) == 0 {
		return nil
	}
	c := candles[len(candles)-1]
	return &c
}

func (m *Manager) isStale(symbol, tf string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sym, ok := m.data[symbol]
	if !ok {
		return true
	}
	e, ok := sym[tf]
	if !ok {
		return true
	}
	ttl, ok := ttlByTF[tf]
	if !ok {
		ttl = 30 * time.Second
	}
	return time.Since(e.fetchedAt) >= ttl
}

// SetTF сохраняет свечи для конкретного таймфрейма (вызывается воркером).
func (m *Manager) SetTF(symbol, tf string, data []api.Candle) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data[symbol] == nil {
		m.data[symbol] = make(map[string]entry)
	}
	m.data[symbol][tf] = entry{candles: data, fetchedAt: time.Now()}
}

func (m *Manager) set(symbol, tf string, data []api.Candle) { m.SetTF(symbol, tf, data) }
