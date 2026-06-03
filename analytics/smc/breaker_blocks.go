// Пакет smc — Smart Money Concepts.
// breaker_blocks.go — Breaker Block: OB, потерявший свою функцию и ставший поддержкой/сопротивлением.
package smc

import "bybit-elite-signal/api"

// BreakerBlock — бывший OB, пробитый ценой.
type BreakerBlock struct {
	OriginalType OBType
	Top          float64
	Bottom       float64
	Age          int
	Quality      float64
}

// DetectBreakerBlocks находит Breaker Blocks (OB, через которые прошла цена).
func DetectBreakerBlocks(candles []api.Candle, maxAge int) []BreakerBlock {
	obs := DetectOrderBlocks(candles, maxAge)
	result := make([]BreakerBlock, 0)
	if len(candles) == 0 {
		return result
	}
	last := candles[len(candles)-1]
	lastC, _ := last.Close.Float64()

	for _, ob := range obs {
		mid := (ob.Top + ob.Bottom) / 2
		// Бычий OB пробит вниз → стал Bearish Breaker
		if ob.Type == BullishOB && lastC < mid && ob.Freshness < 1.0 {
			result = append(result, BreakerBlock{
				OriginalType: ob.Type, Top: ob.Top, Bottom: ob.Bottom,
				Age: ob.Age, Quality: ob.Quality * 0.8,
			})
		}
		// Медвежий OB пробит вверх → стал Bullish Breaker
		if ob.Type == BearishOB && lastC > mid && ob.Freshness < 1.0 {
			result = append(result, BreakerBlock{
				OriginalType: ob.Type, Top: ob.Top, Bottom: ob.Bottom,
				Age: ob.Age, Quality: ob.Quality * 0.8,
			})
		}
	}
	return result
}
