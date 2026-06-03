// Пакет liquidity — анализ ликвидности.
// sweep.go — обнаружение снятия ликвидности (Liquidity Sweep).
package liquidity

import "bybit-elite-signal/api"

// SweepResult — результат обнаружения снятия ликвидности.
type SweepResult struct {
	Detected  bool
	Direction string  // "BullishSweep" (sweep lows) | "BearishSweep" (sweep highs)
	Level     float64 // снятый уровень
	SweptAt   float64 // экстремум свипа
	Confirmed bool    // закрылся обратно за уровень
}

// DetectSweep ищет снятие ликвидности на последней свече.
func DetectSweep(candles []api.Candle, highs, lows []LiquidityLevel) *SweepResult {
	r := &SweepResult{}
	if len(candles) < 2 {
		return r
	}
	last := candles[len(candles)-1]
	lastH, _ := last.High.Float64()
	lastL, _ := last.Low.Float64()
	lastC, _ := last.Close.Float64()

	// Снятие Buy Side Liquidity (highs) = медвежий паттерн
	for _, lvl := range highs {
		if lastH > lvl.Price && lastC < lvl.Price {
			r.Detected = true
			r.Direction = "BearishSweep"
			r.Level = lvl.Price
			r.SweptAt = lastH
			r.Confirmed = true
			return r
		}
	}

	// Снятие Sell Side Liquidity (lows) = бычий паттерн
	for _, lvl := range lows {
		if lastL < lvl.Price && lastC > lvl.Price {
			r.Detected = true
			r.Direction = "BullishSweep"
			r.Level = lvl.Price
			r.SweptAt = lastL
			r.Confirmed = true
			return r
		}
	}
	return r
}
