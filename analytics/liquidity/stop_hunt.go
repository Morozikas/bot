// Пакет liquidity — анализ ликвидности.
// stop_hunt.go — обнаружение охоты на стопы (Stop Hunt / False Breakout).
package liquidity

import "bybit-elite-signal/api"

// StopHunt — обнаруженная охота за стопами.
type StopHunt struct {
	Direction   string  // "Long" | "Short" (направление ловушки)
	Price       float64 // уровень, где стопы собраны
	CandleIdx   int
	IsConfirmed bool
}

// DetectStopHunts находит свечи с признаками охоты за стопами.
func DetectStopHunts(candles []api.Candle) []StopHunt {
	hunts := make([]StopHunt, 0)
	for i := 3; i < len(candles)-1; i++ {
		h, _ := candles[i].High.Float64()
		l, _ := candles[i].Low.Float64()
		cl, _ := candles[i].Close.Float64()
		nextCl, _ := candles[i+1].Close.Float64()

		swingH, swingL := swingRange(candles[i-3:i])

		// Медвежья охота: спайк выше свинга, закрылся обратно
		if h > swingH && cl < swingH && nextCl < cl {
			hunts = append(hunts, StopHunt{
				Direction: "Long", Price: h, CandleIdx: i, IsConfirmed: true,
			})
		}

		// Бычья охота: спайк ниже свинга, закрылся обратно
		if l < swingL && cl > swingL && nextCl > cl {
			hunts = append(hunts, StopHunt{
				Direction: "Short", Price: l, CandleIdx: i, IsConfirmed: true,
			})
		}
	}
	return hunts
}

func swingRange(candles []api.Candle) (high, low float64) {
	low = 1e15
	for _, c := range candles {
		h, _ := c.High.Float64()
		l, _ := c.Low.Float64()
		if h > high {
			high = h
		}
		if l < low {
			low = l
		}
	}
	return
}
