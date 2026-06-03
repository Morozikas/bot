// Пакет heatmap — анализ тепловой карты стакана.
// spoofing.go — обнаружение спуфинга: крупные ордера, исчезающие без исполнения.
package heatmap

import "bybit-elite-signal/api"

// SpoofResult — результат обнаружения манипуляций.
type SpoofResult struct {
	Detected    bool
	BidSpoof    bool
	AskSpoof    bool
	Confidence  float64 // 0-100
}

// DetectSpoofing обнаруживает потенциальный спуфинг по текущему стакану.
// sensitivity: 0-1, выше = чувствительнее.
func DetectSpoofing(ob *api.OrderBook, sensitivity float64) *SpoofResult {
	r := &SpoofResult{}
	if ob == nil {
		return r
	}
	bidWalls := walls(ob.Bids)
	askWalls := walls(ob.Asks)

	bidTotal, askTotal := sumWalls(bidWalls), sumWalls(askWalls)
	if askTotal == 0 {
		return r
	}
	ratio := bidTotal / askTotal
	threshold := 1.0 + (1.0-sensitivity)*4
	if ratio > threshold {
		r.BidSpoof = true
		r.Detected = true
		r.Confidence = clampH((ratio-threshold)*20, 0, 100)
	}
	if 1/ratio > threshold {
		r.AskSpoof = true
		r.Detected = true
		r.Confidence = clampH((1/ratio-threshold)*20, 0, 100)
	}
	return r
}

func walls(levels []api.OrderBookLevel) []float64 {
	if len(levels) == 0 {
		return nil
	}
	avg := 0.0
	for _, l := range levels {
		v, _ := l.Quantity.Float64()
		avg += v
	}
	avg /= float64(len(levels))
	result := make([]float64, 0)
	for _, l := range levels {
		v, _ := l.Quantity.Float64()
		if v > avg*5 {
			result = append(result, v)
		}
	}
	return result
}

func sumWalls(walls []float64) float64 {
	s := 0.0
	for _, w := range walls {
		s += w
	}
	return s
}

func clampH(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
