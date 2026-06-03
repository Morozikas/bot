// Пакет heatmap — анализ тепловой карты стакана.
// iceberg.go — обнаружение айсберг-ордеров: скрытый объём за видимым.
package heatmap

import "bybit-elite-signal/api"

// IcebergSignal — признак айсберг-ордера.
type IcebergSignal struct {
	Price         float64
	Side          string
	VisibleVol    float64
	EstimatedHidden float64
}

// DetectIcebergs ищет признаки айсбергов (постоянное пополнение уровня в сделках vs стакан).
// Сравнивает объём торгов на уровне с видимым объёмом стакана.
func DetectIcebergs(ob *api.OrderBook, trades []api.Trade) []IcebergSignal {
	if ob == nil {
		return nil
	}
	// Строим карту объёма торгов на каждом уровне
	tradedVolByPrice := make(map[float64]float64)
	for _, t := range trades {
		p, _ := t.Price.Float64()
		q, _ := t.Quantity.Float64()
		tradedVolByPrice[roundTo(p, 4)] += q
	}

	result := make([]IcebergSignal, 0)
	for _, ask := range ob.Asks {
		p, _ := ask.Price.Float64()
		visible, _ := ask.Quantity.Float64()
		traded := tradedVolByPrice[roundTo(p, 4)]
		// Если торгов в 3+ раз больше видимого = скрытый объём
		if traded > visible*3 && visible > 0 {
			result = append(result, IcebergSignal{
				Price: p, Side: "Ask",
				VisibleVol: visible, EstimatedHidden: traded - visible,
			})
		}
	}
	return result
}

func roundTo(v float64, decimals int) float64 {
	p := 1.0
	for i := 0; i < decimals; i++ {
		p *= 10
	}
	return float64(int(v*p+0.5)) / p
}
