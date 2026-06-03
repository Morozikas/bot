// Пакет scanners — сканирование рынка.
// volatility.go — фильтрация по волатильности: ATR-режим, компрессия, экспансия.
package scanners

import (
	"math"

	"bybit-elite-signal/api"
)

// VolatilityFilter проверяет, допустима ли торговля по условию волатильности.
type VolatilityFilter struct {
	MinATRRatio float64 // min ATR5/ATR14 (компрессия ниже этого = нет сделки)
}

// NewVolatilityFilter создаёт фильтр.
func NewVolatilityFilter(minRatio float64) *VolatilityFilter {
	return &VolatilityFilter{MinATRRatio: minRatio}
}

// IsAllowed возвращает true если волатильность допускает торговлю.
func (f *VolatilityFilter) IsAllowed(candles []api.Candle) bool {
	atr14 := atrN(candles, 14)
	atr5 := atrN(candles, 5)
	if atr14 == 0 {
		return false
	}
	return atr5/atr14 >= f.MinATRRatio
}

// ATR14 вычисляет ATR(14).
func ATR14(candles []api.Candle) float64 { return atrN(candles, 14) }

func atrN(candles []api.Candle, n int) float64 {
	if len(candles) < n+1 {
		return 0
	}
	src := candles[len(candles)-n-1:]
	total := 0.0
	for i := 1; i <= n; i++ {
		h, _ := src[i].High.Float64()
		l, _ := src[i].Low.Float64()
		pc, _ := src[i-1].Close.Float64()
		tr := math.Max(h-l, math.Max(math.Abs(h-pc), math.Abs(l-pc)))
		total += tr
	}
	return total / float64(n)
}
