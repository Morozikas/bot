// Пакет structure — анализ рыночной структуры.
// swings.go — обнаружение свинговых точек (пивотных максимумов и минимумов).
package structure

import "bybit-elite-signal/api"

// SwingPoint — пивотная точка рынка.
type SwingPoint struct {
	Index int
	Price float64
	IsHigh bool
}

// DetectSwings находит свинговые максимумы и минимумы (pivot-N метод).
func DetectSwings(candles []api.Candle, n int) (highs, lows []SwingPoint) {
	highs = make([]SwingPoint, 0)
	lows = make([]SwingPoint, 0)
	for i := n; i < len(candles)-n; i++ {
		h, _ := candles[i].High.Float64()
		l, _ := candles[i].Low.Float64()
		isHigh, isLow := true, true
		for j := i - n; j <= i+n; j++ {
			if j == i {
				continue
			}
			jh, _ := candles[j].High.Float64()
			jl, _ := candles[j].Low.Float64()
			if jh >= h {
				isHigh = false
			}
			if jl <= l {
				isLow = false
			}
		}
		if isHigh {
			highs = append(highs, SwingPoint{Index: i, Price: h, IsHigh: true})
		}
		if isLow {
			lows = append(lows, SwingPoint{Index: i, Price: l, IsHigh: false})
		}
	}
	return
}

// LastN возвращает последние n элементов.
func LastN(points []SwingPoint, n int) []SwingPoint {
	if len(points) <= n {
		return points
	}
	return points[len(points)-n:]
}
