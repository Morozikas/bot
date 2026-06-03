// Пакет liquidity — анализ ликвидности.
// equal_highs.go — обнаружение равных максимумов (потенциальная Buy Side Liquidity).
package liquidity

import "bybit-elite-signal/api"

// EqualHighs находит уровни с равными максимумами в пределах допуска.
func EqualHighs(candles []api.Candle, tolerance float64) []LiquidityLevel {
	return findEqualLevels(candles, "High", tolerance)
}

// EqualLows находит уровни с равными минимумами.
func EqualLows(candles []api.Candle, tolerance float64) []LiquidityLevel {
	return findEqualLevels(candles, "Low", tolerance)
}

func findEqualLevels(candles []api.Candle, side string, tolerance float64) []LiquidityLevel {
	levels := make([]LiquidityLevel, 0)
	for i := len(candles) - 1; i >= 1; i-- {
		var pi float64
		if side == "High" {
			pi, _ = candles[i].High.Float64()
		} else {
			pi, _ = candles[i].Low.Float64()
		}
		if pi == 0 {
			continue
		}
		count := 1
		for j := i - 1; j >= 0; j-- {
			var pj float64
			if side == "High" {
				pj, _ = candles[j].High.Float64()
			} else {
				pj, _ = candles[j].Low.Float64()
			}
			diff := pi - pj
			if diff < 0 {
				diff = -diff
			}
			if diff/pi <= tolerance {
				count++
			}
		}
		if count >= 2 {
			ltype := "EqualHigh"
			if side == "Low" {
				ltype = "EqualLow"
			}
			levels = append(levels, LiquidityLevel{
				Price:    pi,
				Type:     ltype,
				Strength: float64(count) * 25,
				Tested:   count,
			})
		}
	}
	return deduplicateLvl(levels, tolerance*5)
}
