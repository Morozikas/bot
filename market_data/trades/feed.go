// Пакет trades — поток публичных сделок.
// feed.go — хранение и обновление ленты последних сделок по символам.
package trades

import (
	"sync"

	"bybit-elite-signal/api"
	"go.uber.org/zap"
)

// Feed хранит последние N сделок по каждому символу.
type Feed struct {
	client *api.Client
	log    *zap.Logger
	mu     sync.RWMutex
	data   map[string][]api.Trade
	maxLen int
}

// NewFeed создаёт Feed.
func NewFeed(client *api.Client, log *zap.Logger, maxLen int) *Feed {
	if maxLen <= 0 {
		maxLen = 500
	}
	return &Feed{client: client, log: log, data: make(map[string][]api.Trade), maxLen: maxLen}
}

// Fetch загружает последние сделки для символа.
func (f *Feed) Fetch(symbol string, limit int) {
	trades, err := f.client.GetRecentTrades(symbol, limit)
	if err != nil {
		f.log.Warn("ошибка ленты сделок", zap.String("symbol", symbol), zap.Error(err))
		return
	}
	f.mu.Lock()
	f.data[symbol] = trades
	f.mu.Unlock()
}

// Add добавляет сделку из WS в ленту (держит последние maxLen).
func (f *Feed) Add(symbol string, trade api.Trade) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data[symbol] = append(f.data[symbol], trade)
	if len(f.data[symbol]) > f.maxLen {
		f.data[symbol] = f.data[symbol][len(f.data[symbol])-f.maxLen:]
	}
}

// Get возвращает копию ленты сделок.
func (f *Feed) Get(symbol string) []api.Trade {
	f.mu.RLock()
	defer f.mu.RUnlock()
	src := f.data[symbol]
	if len(src) == 0 {
		return nil
	}
	cp := make([]api.Trade, len(src))
	copy(cp, src)
	return cp
}
