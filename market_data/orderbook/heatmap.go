// Пакет orderbook — управление данными стакана.
// heatmap.go — построение тепловой карты ликвидности из стакана.
package orderbook

import (
	"bybit-elite-signal/api"
	"github.com/shopspring/decimal"
)

// HeatmapLevel — уровень тепловой карты.
type HeatmapLevel struct {
	Price    float64
	BidSize  float64
	AskSize  float64
	Total    float64
	IsWall   bool // объём > 5x среднего
}

// BuildHeatmap строит тепловую карту из снимка стакана.
func BuildHeatmap(ob *api.OrderBook) []HeatmapLevel {
	if ob == nil {
		return nil
	}
	levels := make(map[float64]*HeatmapLevel)

	add := func(lvl api.OrderBookLevel, side string) {
		p, _ := lvl.Price.Float64()
		q, _ := lvl.Quantity.Float64()
		if _, ok := levels[p]; !ok {
			levels[p] = &HeatmapLevel{Price: p}
		}
		if side == "bid" {
			levels[p].BidSize += q
		} else {
			levels[p].AskSize += q
		}
		levels[p].Total = levels[p].BidSize + levels[p].AskSize
	}

	for _, b := range ob.Bids {
		add(b, "bid")
	}
	for _, a := range ob.Asks {
		add(a, "ask")
	}

	// Вычисляем среднее для определения "стен"
	total := decimal.Zero
	count := 0
	for _, l := range levels {
		total = total.Add(decimal.NewFromFloat(l.Total))
		count++
	}
	avg := 0.0
	if count > 0 {
		avg, _ = total.Div(decimal.NewFromInt(int64(count))).Float64()
	}

	result := make([]HeatmapLevel, 0, len(levels))
	for _, l := range levels {
		if avg > 0 && l.Total > avg*5 {
			l.IsWall = true
		}
		result = append(result, *l)
	}
	return result
}
