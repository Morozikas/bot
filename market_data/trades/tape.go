// Пакет trades — поток публичных сделок.
// tape.go — агрегация ленты сделок: суммарный объём покупок/продаж, крупные сделки.
package trades

import "bybit-elite-signal/api"

// TapeStats — агрегированная статистика ленты сделок.
type TapeStats struct {
	BuyVolume    float64
	SellVolume   float64
	BuyCount     int
	SellCount    int
	TotalVolume  float64
	BuySellRatio float64 // > 1 = доминируют покупки
	LargeBuys    []api.Trade // сделки объёмом > порога
	LargeSells   []api.Trade
}

// Aggregate вычисляет статистику по ленте сделок.
// largeThresholdUSDT: порог в USDT для классификации крупной сделки.
func Aggregate(trades []api.Trade, largeThresholdUSDT float64) *TapeStats {
	s := &TapeStats{}
	for _, t := range trades {
		p, _ := t.Price.Float64()
		q, _ := t.Quantity.Float64()
		notional := p * q
		if t.IsBuyer {
			s.BuyVolume += q
			s.BuyCount++
			if notional >= largeThresholdUSDT {
				s.LargeBuys = append(s.LargeBuys, t)
			}
		} else {
			s.SellVolume += q
			s.SellCount++
			if notional >= largeThresholdUSDT {
				s.LargeSells = append(s.LargeSells, t)
			}
		}
	}
	s.TotalVolume = s.BuyVolume + s.SellVolume
	if s.SellVolume > 0 {
		s.BuySellRatio = s.BuyVolume / s.SellVolume
	}
	return s
}
