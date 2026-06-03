// Пакет footprint — анализ ордер-флоу.
// delta.go — дельта (AskVol - BidVol) и её дивергенция с ценой.
package footprint

import "bybit-elite-signal/api"

// DeltaCandle — дельта одной свечи.
type DeltaCandle struct {
	Open     float64
	Close    float64
	BidVol   float64
	AskVol   float64
	Delta    float64
	CumDelta float64
}

// CalcDelta вычисляет дельту для серии свечей из публичных сделок.
func CalcDelta(candles []api.Candle, trades []api.Trade) []DeltaCandle {
	result := make([]DeltaCandle, len(candles))
	tradeMap := mapTrades(trades, candles)
	cumDelta := 0.0

	for i, c := range candles {
		o, _ := c.Open.Float64()
		cl, _ := c.Close.Float64()
		v, _ := c.Volume.Float64()
		dc := DeltaCandle{Open: o, Close: cl}

		if bucket, ok := tradeMap[i]; ok {
			for _, t := range bucket {
				qty, _ := t.Quantity.Float64()
				if t.IsBuyer {
					dc.AskVol += qty
				} else {
					dc.BidVol += qty
				}
			}
		} else {
			body := cl - o
			if body >= 0 {
				dc.AskVol = v * 0.6
				dc.BidVol = v * 0.4
			} else {
				dc.AskVol = v * 0.4
				dc.BidVol = v * 0.6
			}
		}
		dc.Delta = dc.AskVol - dc.BidVol
		cumDelta += dc.Delta
		dc.CumDelta = cumDelta
		result[i] = dc
	}
	return result
}

// DeltaDivergence обнаруживает расхождение цены и дельты.
func DeltaDivergence(dc []DeltaCandle) bool {
	if len(dc) < 3 {
		return false
	}
	last := dc[len(dc)-1]
	prev := dc[len(dc)-2]
	if last.Close > prev.Close && last.Delta < prev.Delta && last.Delta < 0 {
		return true
	}
	if last.Close < prev.Close && last.Delta > prev.Delta && last.Delta > 0 {
		return true
	}
	return false
}

func mapTrades(trades []api.Trade, candles []api.Candle) map[int][]api.Trade {
	result := make(map[int][]api.Trade)
	for _, t := range trades {
		for i, c := range candles {
			if i == len(candles)-1 {
				result[i] = append(result[i], t)
				break
			}
			if t.Timestamp >= c.StartTime && t.Timestamp < candles[i+1].StartTime {
				result[i] = append(result[i], t)
				break
			}
		}
	}
	return result
}
