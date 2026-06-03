// Пакет footprint — анализ ордер-флоу.
// imbalance.go — bid/ask дисбалансы и стековые дисбалансы внутри свечи.
package footprint

// Imbalance — дисбаланс bid/ask на уровне свечи.
type Imbalance struct {
	Side  string  // "Ask" | "Bid"
	Ratio float64
	Price float64
}

// DetectImbalances находит дисбалансы с порогом threshold (напр. 3.0 = 3:1).
func DetectImbalances(dc []DeltaCandle, threshold float64) []Imbalance {
	result := make([]Imbalance, 0)
	for _, d := range dc {
		if d.BidVol > 0 && d.AskVol/d.BidVol >= threshold {
			result = append(result, Imbalance{Side: "Ask", Ratio: d.AskVol / d.BidVol, Price: d.Close})
		}
		if d.AskVol > 0 && d.BidVol/d.AskVol >= threshold {
			result = append(result, Imbalance{Side: "Bid", Ratio: d.BidVol / d.AskVol, Price: d.Close})
		}
	}
	return result
}

// StackedImbalances считает кол-во последовательных имбалансов в одном направлении.
func StackedImbalances(dc []DeltaCandle, threshold float64) int {
	imbalances := DetectImbalances(dc, threshold)
	maxStack, curStack, lastSide := 0, 0, ""
	for _, imb := range imbalances {
		if imb.Side == lastSide {
			curStack++
		} else {
			if curStack > maxStack {
				maxStack = curStack
			}
			curStack = 1
			lastSide = imb.Side
		}
	}
	if curStack > maxStack {
		maxStack = curStack
	}
	return maxStack
}
