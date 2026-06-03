// Пакет heatmap — анализ тепловой карты стакана.
// score.go — Heatmap Score (0-100) на основе стен, имбалансов, спуфинга.
package heatmap

import "bybit-elite-signal/api"

// HeatmapResult — итог анализа стакана.
type HeatmapResult struct {
	Walls     []Wall
	Vacuums   []LiquidityVacuum
	Icebergs  []IcebergSignal
	Spoof     *SpoofResult
	BidAskImb float64 // > 0 = больше бидов
	Score     float64
}

// Analyze выполняет полный анализ тепловой карты.
func Analyze(ob *api.OrderBook, trades []api.Trade, spoofSensitivity float64) *HeatmapResult {
	r := &HeatmapResult{}
	if ob == nil {
		return r
	}
	r.Walls = FindWalls(ob)
	r.Vacuums = FindVacuums(ob)
	r.Icebergs = DetectIcebergs(ob, trades)
	r.Spoof = DetectSpoofing(ob, spoofSensitivity)

	// Bid/Ask имбаланс
	bidTotal, askTotal := 0.0, 0.0
	for _, b := range ob.Bids {
		q, _ := b.Quantity.Float64()
		p, _ := b.Price.Float64()
		bidTotal += q * p
	}
	for _, a := range ob.Asks {
		q, _ := a.Quantity.Float64()
		p, _ := a.Price.Float64()
		askTotal += q * p
	}
	if total := bidTotal + askTotal; total > 0 {
		r.BidAskImb = (bidTotal - askTotal) / total
	}

	score := 50.0
	if r.BidAskImb > 0.2 {
		score += 15
	} else if r.BidAskImb < -0.2 {
		score -= 15
	}
	if r.Spoof != nil && r.Spoof.Detected {
		score -= 30
	}
	if len(r.Vacuums) > 0 {
		score += 10
	}
	r.Score = clampH(score, 0, 100)
	return r
}
