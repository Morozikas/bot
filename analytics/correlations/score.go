// Пакет correlations — анализ корреляций.
// score.go — Correlation Score.
package correlations

// Score возвращает корректировку оценки сигнала на основе корреляций.
func Score(r *CorrResult) float64 {
	if r == nil {
		return 0
	}
	return r.ScoreAdj
}

// dominance.go — Доминация BTC/USDT (stub).

// DominanceTrend — тренд доминации.
type DominanceTrend struct {
	BTCDomTrend   string // "Rising" | "Falling" | "Neutral"
	USDTDomTrend  string
}

// AnalyzeDominance — заглушка; в реальности требует данных TradingView/CoinGecko.
func AnalyzeDominance() *DominanceTrend {
	return &DominanceTrend{BTCDomTrend: "Neutral", USDTDomTrend: "Neutral"}
}
