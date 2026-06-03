// Пакет scanners — сканирование и ранжирование рынка.
// scanner.go — один вызов GetTickers, обновление Baseline, ранжирование.
package scanners

import (
	"bybit-elite-signal/api"
	"bybit-elite-signal/config"
	"go.uber.org/zap"
)

// ScanResult — результат сканирования: ранг + кеш тикеров.
type ScanResult struct {
	Scores  []SymbolScore
	Tickers map[string]*api.Ticker
}

// MarketScanner — центральный сканер рынка.
type MarketScanner struct {
	client   *api.Client
	cfg      *config.SignalConfig
	log      *zap.Logger
	baseline *Baseline // исторические средние по объёму и OI
}

// NewMarketScanner создаёт MarketScanner.
func NewMarketScanner(client *api.Client, cfg *config.SignalConfig, log *zap.Logger) *MarketScanner {
	return &MarketScanner{
		client:   client,
		cfg:      cfg,
		log:      log,
		baseline: NewBaseline(),
	}
}

// Scan выполняет ОДИН вызов GetTickers, обновляет Baseline и возвращает ранжирование.
func (s *MarketScanner) Scan() *ScanResult {
	tickers, err := s.client.GetTickers("")
	if err != nil {
		s.log.Error("ошибка тикеров при сканировании", zap.Error(err))
		return nil
	}

	// Обновляем историческое среднее для каждой монеты
	for _, t := range tickers {
		vol, _ := t.Turnover24h.Float64()
		oi, _ := t.OpenInterestVal.Float64()
		s.baseline.Update(t.Symbol, vol, oi)
	}

	histLen := s.baseline.HistoryLen("BTCUSDT")
	s.log.Debug("baseline обновлён",
		zap.Int("symbols", len(tickers)),
		zap.Int("history_depth", histLen),
	)

	scores := RankSymbols(tickers, s.cfg.TopCoinsLimit, s.baseline)

	// Индексируем тикеры по символу
	tickerMap := make(map[string]*api.Ticker, len(tickers))
	for i := range tickers {
		t := &tickers[i]
		tickerMap[t.Symbol] = t
	}

	return &ScanResult{Scores: scores, Tickers: tickerMap}
}

// Symbols возвращает имена символов из результата ранжирования.
func Symbols(scores []SymbolScore) []string {
	result := make([]string, 0, len(scores))
	for _, s := range scores {
		result = append(result, s.Symbol)
	}
	return result
}
