// Пакет structure — анализ рыночной структуры.
// score.go — расчёт Structure Score (0-100) на основе обнаруженных паттернов.
package structure

// StructureScore вычисляет итоговую оценку рыночной структуры.
func StructureScore(r *TrendResult) float64 {
	if r == nil {
		return 0
	}
	score := 50.0
	switch r.Regime {
	case RegimeCHOCH:
		score += 30
	case RegimeBOS:
		score += 20
	case RegimeBullish, RegimeBearish:
		score += 10
	case RegimeExpansion:
		score += 15
	case RegimeCompression:
		score = 10 // торговля при компрессии запрещена
		return score
	}
	if r.BOS != nil && r.BOS.Detected {
		score += 10
	}
	if r.CHOCH != nil && r.CHOCH.Detected {
		score += 15
	}
	return clampS(score, 0, 100)
}
