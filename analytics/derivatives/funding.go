// Пакет derivatives — аналитика деривативов.
// funding.go — ставка финансирования: классификация и влияние на оценку сигнала.
package derivatives

import "bybit-elite-signal/api"

// FundingCondition — состояние funding rate.
type FundingCondition string

const (
	ExtremePositive FundingCondition = "ExtremePositive"
	Positive        FundingCondition = "Positive"
	Neutral         FundingCondition = "Neutral"
	Negative        FundingCondition = "Negative"
	ExtremeNegative FundingCondition = "ExtremeNegative"
)

// FundingResult — результат анализа funding rate.
type FundingResult struct {
	CurrentRate  float64
	Condition    FundingCondition
	ScoreAdj     float64
	IsOverheated bool
}

// AnalyzeFunding обрабатывает историю funding rate.
func AnalyzeFunding(history []api.FundingRate, extremePos, extremeNeg float64) *FundingResult {
	r := &FundingResult{}
	if len(history) == 0 {
		return r
	}
	rate, _ := history[0].FundingRate.Float64()
	r.CurrentRate = rate
	switch {
	case rate >= extremePos:
		r.Condition = ExtremePositive
		r.ScoreAdj = -15
		r.IsOverheated = true
	case rate >= extremePos/2:
		r.Condition = Positive
		r.ScoreAdj = -5
	case rate <= extremeNeg:
		r.Condition = ExtremeNegative
		r.ScoreAdj = -15
		r.IsOverheated = true
	case rate <= extremeNeg/2:
		r.Condition = Negative
		r.ScoreAdj = -5
	default:
		r.Condition = Neutral
	}
	return r
}

func clampD(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
