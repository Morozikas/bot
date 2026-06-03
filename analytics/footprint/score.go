// Пакет footprint — анализ ордер-флоу.
// score.go — Footprint Score (0-100) и итоговое направление ордер-флоу.
package footprint

import "bybit-elite-signal/api"

// FootprintResult — полный результат анализа ордер-флоу.
type FootprintResult struct {
	Candles          []DeltaCandle
	CVD              *CVDAnalysis
	Imbalances       []Imbalance
	Absorptions      []Absorption
	Exhaustions      []Exhaustion
	StackedImbalances int
	Score            float64
	Direction        string // "Bullish" | "Bearish" | "Neutral"
}

// Analyze выполняет полный анализ ордер-флоу.
func Analyze(candles []api.Candle, trades []api.Trade, imbalanceThresh float64) *FootprintResult {
	r := &FootprintResult{}
	if len(candles) < 3 {
		return r
	}
	r.Candles = CalcDelta(candles, trades)
	r.CVD = AnalyzeCVD(r.Candles)
	r.Imbalances = DetectImbalances(r.Candles, imbalanceThresh)
	r.Absorptions = DetectAbsorption(r.Candles)
	r.Exhaustions = DetectExhaustion(r.Candles)
	r.StackedImbalances = StackedImbalances(r.Candles, imbalanceThresh)

	switch {
	case hasBullishSignals(r):
		r.Direction = "Bullish"
	case hasBearishSignals(r):
		r.Direction = "Bearish"
	default:
		r.Direction = "Neutral"
	}

	score := 50.0
	if r.CVD != nil && r.CVD.Diverges {
		score += 20
	}
	for _, a := range r.Absorptions {
		if a.Type == "BuyerAbsorption" || a.Type == "SellerAbsorption" {
			score += 15
			break
		}
	}
	if len(r.Exhaustions) > 0 {
		score += 10
	}
	if r.StackedImbalances > 5 {
		score += 10
	}
	r.Score = clampFP(score, 0, 100)
	return r
}

// Score возвращает итоговый Footprint Score.
func Score(r *FootprintResult) float64 {
	if r == nil {
		return 0
	}
	return r.Score
}

func hasBullishSignals(r *FootprintResult) bool {
	for _, a := range r.Absorptions {
		if a.Type == "BuyerAbsorption" {
			return true
		}
	}
	for _, e := range r.Exhaustions {
		if e.Type == "SellerExhaustion" {
			return true
		}
	}
	return r.CVD != nil && r.CVD.Trend == CVDBullish
}

func hasBearishSignals(r *FootprintResult) bool {
	for _, a := range r.Absorptions {
		if a.Type == "SellerAbsorption" {
			return true
		}
	}
	for _, e := range r.Exhaustions {
		if e.Type == "BuyerExhaustion" {
			return true
		}
	}
	return r.CVD != nil && r.CVD.Trend == CVDBearish
}

func clampFP(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
