// Пакет heatmap — анализ тепловой карты стакана.
// walls.go — стены ликвидности и скрытые ордера (айсберги).
package heatmap

import "bybit-elite-signal/api"

// Wall — крупный ордер в стакане.
type Wall struct {
	Price    float64
	Volume   float64
	Side     string // "Bid" | "Ask"
	IsHidden bool   // предполагаемый айсберг
}

// FindWalls находит стены ликвидности (объём > 5x среднего).
func FindWalls(ob *api.OrderBook) []Wall {
	if ob == nil {
		return nil
	}
	result := append(
		findWallsSide(ob.Bids, "Bid"),
		findWallsSide(ob.Asks, "Ask")...,
	)
	return result
}

func findWallsSide(levels []api.OrderBookLevel, side string) []Wall {
	if len(levels) == 0 {
		return nil
	}
	avg := 0.0
	for _, l := range levels {
		v, _ := l.Quantity.Float64()
		avg += v
	}
	avg /= float64(len(levels))
	result := make([]Wall, 0)
	for _, l := range levels {
		v, _ := l.Quantity.Float64()
		p, _ := l.Price.Float64()
		if v > avg*5 {
			result = append(result, Wall{Price: p, Volume: v, Side: side})
		}
	}
	return result
}

// LiquidityVacuum — ценовая зона с почти нулевой глубиной стакана.
type LiquidityVacuum struct {
	From float64
	To   float64
}

// FindVacuums находит разрывы в стакане > 0.1%.
func FindVacuums(ob *api.OrderBook) []LiquidityVacuum {
	if ob == nil || len(ob.Asks) < 2 {
		return nil
	}
	result := make([]LiquidityVacuum, 0)
	for i := 1; i < len(ob.Asks); i++ {
		prev, _ := ob.Asks[i-1].Price.Float64()
		curr, _ := ob.Asks[i].Price.Float64()
		if prev > 0 && (curr-prev)/prev > 0.001 {
			result = append(result, LiquidityVacuum{From: prev, To: curr})
		}
	}
	return result
}
