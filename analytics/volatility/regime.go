// Пакет volatility — анализ волатильности.
// regime.go — режим волатильности: компрессия, нейтраль, экспансия.
package volatility

import "bybit-elite-signal/api"

// Regime — режим волатильности.
type Regime string

const (
	Compression Regime = "Compression"
	Normal      Regime = "Normal"
	Expansion   Regime = "Expansion"
)

// VolResult — полный результат анализа волатильности.
type VolResult struct {
	ATR14           float64
	ATR5            float64
	RealizedVol20   float64
	RealizedVol5    float64
	CompressionRatio float64
	Regime          Regime
	IsCompressed    bool
	IsExpanding     bool
	NormATR         float64
}

// Analyze выполняет полный анализ волатильности.
func Analyze(candles []api.Candle) *VolResult {
	r := &VolResult{}
	if len(candles) < 15 {
		return r
	}
	r.ATR14 = ATR14(candles)
	r.ATR5 = ATR5(candles)
	if r.ATR14 > 0 {
		r.CompressionRatio = r.ATR5 / r.ATR14
		r.IsCompressed = r.CompressionRatio < 0.6
		r.IsExpanding = r.CompressionRatio > 1.4
	}
	r.RealizedVol20 = RealizedVol(candles, 20)
	r.RealizedVol5 = RealizedVol(candles, 5)
	r.NormATR = NormalizedATR(candles, 14)

	switch {
	case r.IsCompressed:
		r.Regime = Compression
	case r.IsExpanding:
		r.Regime = Expansion
	default:
		r.Regime = Normal
	}
	return r
}

// Score возвращает Volatility Score (0-100).
func Score(r *VolResult) float64 {
	if r == nil || r.IsCompressed {
		return 0 // компрессия = нет торговли
	}
	if r.IsExpanding {
		return 100
	}
	return 50
}
