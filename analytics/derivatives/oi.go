// Пакет derivatives — аналитика деривативов.
// oi.go — открытый интерес: Long Build-Up, Short Build-Up, Unwinding, Covering.
package derivatives

import "bybit-elite-signal/api"

// OICondition — состояние открытого интереса.
type OICondition string

const (
	LongBuildUp   OICondition = "LongBuildUp"
	ShortBuildUp  OICondition = "ShortBuildUp"
	LongUnwinding OICondition = "LongUnwinding"
	ShortCovering OICondition = "ShortCovering"
	OINeutral     OICondition = "Neutral"
)

// OIResult — результат анализа OI.
type OIResult struct {
	Current     float64
	Previous    float64
	ChangePct   float64
	Condition   OICondition
	AllowsLong  bool
	AllowsShort bool
	Score       float64
}

// AnalyzeOI вычисляет состояние OI.
func AnalyzeOI(oiHistory []api.OpenInterest, candles []api.Candle) *OIResult {
	r := &OIResult{}
	if len(oiHistory) < 2 || len(candles) < 2 {
		return r
	}
	curr, _ := oiHistory[0].OpenInterest.Float64()
	prev, _ := oiHistory[1].OpenInterest.Float64()
	r.Current = curr
	r.Previous = prev
	if prev != 0 {
		r.ChangePct = (curr - prev) / prev * 100
	}
	lc, _ := candles[len(candles)-1].Close.Float64()
	pc, _ := candles[len(candles)-2].Close.Float64()
	priceUp := lc > pc
	oiUp := curr > prev
	switch {
	case priceUp && oiUp:
		r.Condition = LongBuildUp
		r.AllowsLong = true
	case !priceUp && oiUp:
		r.Condition = ShortBuildUp
		r.AllowsShort = true
	case priceUp:
		r.Condition = ShortCovering
	default:
		r.Condition = LongUnwinding
	}
	score := 50.0
	if r.Condition == LongBuildUp || r.Condition == ShortBuildUp {
		score += 30
	} else {
		score -= 10
	}
	if r.ChangePct > 5 {
		score += 10
	} else if r.ChangePct < -5 {
		score -= 10
	}
	r.Score = clampD(score, 0, 100)
	return r
}
