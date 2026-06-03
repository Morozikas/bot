// Пакет liquidity — анализ ликвидности.
// score.go — расчёт Liquidity Score (0-100).
package liquidity

// Score вычисляет итоговую оценку ликвидности.
func Score(m *LiquidityMap) float64 {
	if m == nil {
		return 0
	}
	score := 0.0
	if m.HasLiquidity {
		score += 40
	}
	if m.Sweep != nil && m.Sweep.Detected && m.Sweep.Confirmed {
		score += 40
	}
	score += float64(len(m.StopHunts)) * 5
	score += float64(len(m.LiquidationClusters)) * 3
	if score > 100 {
		return 100
	}
	return score
}

// NearestBuySide возвращает ближайший уровень Buy Side Liquidity выше цены.
func NearestBuySide(m *LiquidityMap, price float64) float64 {
	best := 0.0
	for _, l := range m.BuySide {
		if l.Price > price && (best == 0 || l.Price < best) {
			best = l.Price
		}
	}
	return best
}

// NearestSellSide возвращает ближайший уровень Sell Side Liquidity ниже цены.
func NearestSellSide(m *LiquidityMap, price float64) float64 {
	best := 0.0
	for _, l := range m.SellSide {
		if l.Price < price && (best == 0 || l.Price > best) {
			best = l.Price
		}
	}
	return best
}
