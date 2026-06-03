// Пакет structure — анализ рыночной структуры.
// bos.go — Break of Structure: прорыв ключевого уровня в направлении тренда.
package structure

import "bybit-elite-signal/api"

// BOSResult — результат обнаружения BOS.
type BOSResult struct {
	Detected  bool
	Direction string  // "Bullish" | "Bearish"
	Level     float64 // пробитый уровень
	Strength  float64 // 0-100
}

// DetectBOS обнаруживает Break of Structure.
func DetectBOS(candles []api.Candle, swingHighs, swingLows []SwingPoint) *BOSResult {
	r := &BOSResult{}
	if len(candles) < 3 || len(swingHighs) < 1 || len(swingLows) < 1 {
		return r
	}
	last := candles[len(candles)-1]
	close, _ := last.Close.Float64()

	// Бычий BOS: цена закрытия выше последнего свингового максимума
	lastHigh := swingHighs[len(swingHighs)-1].Price
	if close > lastHigh {
		r.Detected = true
		r.Direction = "Bullish"
		r.Level = lastHigh
		r.Strength = clampS((close-lastHigh)/lastHigh*1000, 0, 100)
		return r
	}

	// Медвежий BOS: цена ниже последнего свингового минимума
	lastLow := swingLows[len(swingLows)-1].Price
	if close < lastLow {
		r.Detected = true
		r.Direction = "Bearish"
		r.Level = lastLow
		r.Strength = clampS((lastLow-close)/lastLow*1000, 0, 100)
	}
	return r
}

func clampS(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
