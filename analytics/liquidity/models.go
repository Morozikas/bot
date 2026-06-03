// Пакет liquidity — анализ ликвидности.
// models.go — общие типы данных пакета.
package liquidity

// LiquidityLevel — ценовой уровень с накопленной ликвидностью.
type LiquidityLevel struct {
	Price    float64
	Type     string  // "EqualHigh" | "EqualLow" | "BuySide" | "SellSide"
	Strength float64 // 0-100
	Tested   int     // сколько раз тестировался
	IsSwept  bool
	SweptAt  float64
}

// Pool — кластер ликвидности.
type Pool struct {
	High     float64
	Low      float64
	Strength float64
	Type     string // "BuySide" | "SellSide"
}

func deduplicateLvl(levels []LiquidityLevel, tol float64) []LiquidityLevel {
	if len(levels) == 0 {
		return levels
	}
	result := []LiquidityLevel{levels[0]}
	for _, l := range levels[1:] {
		dup := false
		for _, r := range result {
			diff := l.Price - r.Price
			if diff < 0 {
				diff = -diff
			}
			if r.Price != 0 && diff/r.Price <= tol {
				dup = true
				break
			}
		}
		if !dup {
			result = append(result, l)
		}
	}
	return result
}
