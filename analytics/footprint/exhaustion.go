// Пакет footprint — анализ ордер-флоу.
// exhaustion.go — истощение покупателей/продавцов: разворотный сигнал ордер-флоу.
package footprint

// Exhaustion — обнаруженное истощение.
type Exhaustion struct {
	Type  string  // "BuyerExhaustion" | "SellerExhaustion"
	Price float64
}

// DetectExhaustion ищет истощение агрессии в последних свечах.
func DetectExhaustion(dc []DeltaCandle) []Exhaustion {
	result := make([]Exhaustion, 0)
	if len(dc) < 3 {
		return result
	}
	last := dc[len(dc)-1]
	prev := dc[len(dc)-2]

	// Истощение покупателей: предыдущая свеча бычья (Ask > Bid), последняя медвежья с отрицательной дельтой
	if prev.AskVol > prev.BidVol && last.Delta < 0 && last.Close < last.Open {
		result = append(result, Exhaustion{Type: "BuyerExhaustion", Price: last.Close})
	}

	// Истощение продавцов: предыдущая свеча медвежья (Bid > Ask), последняя бычья с положительной дельтой
	if prev.BidVol > prev.AskVol && last.Delta > 0 && last.Close > last.Open {
		result = append(result, Exhaustion{Type: "SellerExhaustion", Price: last.Close})
	}
	return result
}
