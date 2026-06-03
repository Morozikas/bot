// Пакет structure — анализ рыночной структуры.
// mss.go — Market Structure Shift: более ранняя смена структуры (на меньшем таймфрейме).
package structure

import "bybit-elite-signal/api"

// MSSResult — результат обнаружения MSS.
type MSSResult struct {
	Detected  bool
	Direction string
	Level     float64
}

// DetectMSS обнаруживает Market Structure Shift.
// MSS — это CHOCH на младшем ТФ, подтверждающий смену на старшем.
func DetectMSS(candles []api.Candle, highs, lows []SwingPoint) *MSSResult {
	r := &MSSResult{}
	if len(candles) < 5 {
		return r
	}
	// MSS: ищем разворот через 3 последних свечи
	// Бычий MSS: цена пробила промежуточный минимум и закрылась выше него
	if len(lows) >= 2 {
		recentLow := lows[len(lows)-1].Price
		cl, _ := candles[len(candles)-1].Close.Float64()
		prev, _ := candles[len(candles)-2].Low.Float64()
		if prev < recentLow && cl > recentLow {
			r.Detected = true
			r.Direction = "Bullish"
			r.Level = recentLow
		}
	}
	if len(highs) >= 2 && !r.Detected {
		recentHigh := highs[len(highs)-1].Price
		cl, _ := candles[len(candles)-1].Close.Float64()
		prev, _ := candles[len(candles)-2].High.Float64()
		if prev > recentHigh && cl < recentHigh {
			r.Detected = true
			r.Direction = "Bearish"
			r.Level = recentHigh
		}
	}
	return r
}
