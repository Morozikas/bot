// Пакет structure — анализ рыночной структуры.
// choch.go — Change of Character: смена структуры против текущего тренда.
package structure

import "bybit-elite-signal/api"

// CHOCHResult — результат обнаружения CHOCH.
type CHOCHResult struct {
	Detected  bool
	Direction string  // "Bullish" | "Bearish" (направление нового потенциального тренда)
	Level     float64
	Strength  float64
}

// DetectCHOCH обнаруживает Change of Character.
// В бычьем тренде CHOCH = пробой последнего HL вниз.
// В медвежьем тренде CHOCH = пробой последнего LH вверх.
func DetectCHOCH(candles []api.Candle, highs, lows []SwingPoint) *CHOCHResult {
	r := &CHOCHResult{}
	if len(candles) < 3 || len(highs) < 2 || len(lows) < 2 {
		return r
	}
	last := candles[len(candles)-1]
	close, _ := last.Close.Float64()

	// Определяем тренд по последним двум свинговым максимумам/минимумам
	lastHH := highs[len(highs)-1].Price
	prevHH := highs[len(highs)-2].Price
	lastLL := lows[len(lows)-1].Price
	prevLL := lows[len(lows)-2].Price

	bullishTrend := lastHH > prevHH && lastLL > prevLL
	bearishTrend := lastHH < prevHH && lastLL < prevLL

	if bullishTrend && close < lastLL {
		r.Detected = true
		r.Direction = "Bearish"
		r.Level = lastLL
		r.Strength = clampS((lastLL-close)/lastLL*1000, 0, 100)
	}
	if bearishTrend && close > lastHH {
		r.Detected = true
		r.Direction = "Bullish"
		r.Level = lastHH
		r.Strength = clampS((close-lastHH)/lastHH*1000, 0, 100)
	}
	return r
}
