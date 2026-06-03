// Пакет derivatives — аналитика деривативов.
// score.go — Derivatives Score: итоговая оценка по OI, funding, L/S ratio.
package derivatives

// DerivativesScore вычисляет итоговую оценку деривативов.
func DerivativesScore(oi *OIResult, funding *FundingResult, ls *LSRatioResult) float64 {
	score := 50.0
	if oi != nil {
		score = oi.Score
	}
	if funding != nil {
		score += funding.ScoreAdj
	}
	if ls != nil {
		score += ls.ScoreAdj
	}
	return clampD(score, 0, 100)
}
