// Пакет volatility — анализ волатильности.
// atr.go — ATR (Average True Range): измерение истинного диапазона цены.
package volatility

import (
	"math"

	"bybit-elite-signal/api"
)

// ATR вычисляет Average True Range за period свечей.
func ATR(candles []api.Candle, period int) float64 {
	if len(candles) < period+1 {
		return 0
	}
	src := candles[len(candles)-period-1:]
	total := 0.0
	for i := 1; i <= period; i++ {
		h, _ := src[i].High.Float64()
		l, _ := src[i].Low.Float64()
		pc, _ := src[i-1].Close.Float64()
		tr := math.Max(h-l, math.Max(math.Abs(h-pc), math.Abs(l-pc)))
		total += tr
	}
	return total / float64(period)
}

// ATR14 возвращает ATR(14).
func ATR14(candles []api.Candle) float64 { return ATR(candles, 14) }

// ATR5 возвращает ATR(5).
func ATR5(candles []api.Candle) float64 { return ATR(candles, 5) }

// NormalizedATR возвращает ATR как % от цены.
func NormalizedATR(candles []api.Candle, period int) float64 {
	a := ATR(candles, period)
	if len(candles) == 0 {
		return 0
	}
	last, _ := candles[len(candles)-1].Close.Float64()
	if last == 0 {
		return 0
	}
	return a / last * 100
}
