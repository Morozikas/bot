// Пакет volatility — анализ волатильности.
// realized.go — Realized и Historical Volatility (аннуализированная).
package volatility

import (
	"math"

	"bybit-elite-signal/api"
)

// RealizedVol вычисляет реализованную волатильность за period свечей (аннуализированная %).
func RealizedVol(candles []api.Candle, period int) float64 {
	if len(candles) < period+1 {
		return 0
	}
	src := candles[len(candles)-period-1:]
	returns := make([]float64, 0, period)
	for i := 1; i <= period; i++ {
		c, _ := src[i].Close.Float64()
		pc, _ := src[i-1].Close.Float64()
		if pc > 0 {
			returns = append(returns, math.Log(c/pc))
		}
	}
	if len(returns) < 2 {
		return 0
	}
	mean := 0.0
	for _, r := range returns {
		mean += r
	}
	mean /= float64(len(returns))
	variance := 0.0
	for _, r := range returns {
		d := r - mean
		variance += d * d
	}
	variance /= float64(len(returns) - 1)
	return math.Sqrt(variance) * math.Sqrt(365) * 100
}

// HistoricalVol вычисляет историческую волатильность за 20 свечей.
func HistoricalVol(candles []api.Candle) float64 { return RealizedVol(candles, 20) }
