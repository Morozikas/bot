// Пакет orderbook — управление данными стакана.
// tracker.go — отслеживание изменений ликвидности между снимками стакана.
package orderbook

import "bybit-elite-signal/api"

// LiquidityChange — изменение ликвидности на уровне цены.
type LiquidityChange struct {
	Price     float64
	Side      string // "bid" | "ask"
	PrevSize  float64
	CurrSize  float64
	Delta     float64
	IsAppear  bool // уровень появился
	IsRemove  bool // уровень исчез (возможный спуфинг)
}

// TrackChanges вычисляет дельту между двумя снимками стакана.
func TrackChanges(prev, curr *api.OrderBook) []LiquidityChange {
	if prev == nil || curr == nil {
		return nil
	}
	changes := make([]LiquidityChange, 0)

	prevBids := toLevelMap(prev.Bids)
	currBids := toLevelMap(curr.Bids)

	for p, prevQ := range prevBids {
		if currQ, ok := currBids[p]; ok {
			if delta := currQ - prevQ; delta != 0 {
				changes = append(changes, LiquidityChange{Price: p, Side: "bid", PrevSize: prevQ, CurrSize: currQ, Delta: delta})
			}
		} else {
			changes = append(changes, LiquidityChange{Price: p, Side: "bid", PrevSize: prevQ, IsRemove: true})
		}
	}
	for p, currQ := range currBids {
		if _, ok := prevBids[p]; !ok {
			changes = append(changes, LiquidityChange{Price: p, Side: "bid", CurrSize: currQ, IsAppear: true})
		}
	}
	return changes
}

func toLevelMap(levels []api.OrderBookLevel) map[float64]float64 {
	m := make(map[float64]float64, len(levels))
	for _, l := range levels {
		p, _ := l.Price.Float64()
		q, _ := l.Quantity.Float64()
		m[p] = q
	}
	return m
}
