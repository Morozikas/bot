// Пакет volume — анализ объёма.
// score.go — Volume Score, VWAP, относительный объём, MA20.
package volume

import "bybit-elite-signal/api"

// VolumeStats — метрики объёма.
type VolumeStats struct {
	Current       float64
	MA20          float64
	RelativeVol   float64
	IsAboveMA20   bool
	IsSpike       bool
	VWAP          float64
	PriceVsVWAP   string // "Above" | "Below"
	Score         float64
}

// Analyze вычисляет объёмные метрики.
func Analyze(candles []api.Candle, minRelVol float64) *VolumeStats {
	s := &VolumeStats{}
	if len(candles) < 21 {
		return s
	}
	cur, _ := candles[len(candles)-1].Volume.Float64()
	s.Current = cur

	sum := 0.0
	for _, c := range candles[len(candles)-21 : len(candles)-1] {
		v, _ := c.Volume.Float64()
		sum += v
	}
	s.MA20 = sum / 20
	if s.MA20 > 0 {
		s.RelativeVol = s.Current / s.MA20
		s.IsAboveMA20 = s.RelativeVol >= minRelVol
		s.IsSpike = s.RelativeVol >= 3.0
	}

	s.VWAP = calcVWAP(candles)
	lc, _ := candles[len(candles)-1].Close.Float64()
	if lc >= s.VWAP {
		s.PriceVsVWAP = "Above"
	} else {
		s.PriceVsVWAP = "Below"
	}

	if s.IsSpike {
		s.Score = 100
	} else if s.IsAboveMA20 {
		s.Score = clampV(60+(s.RelativeVol-1)*20, 0, 100)
	}
	return s
}

// Score рассчитывает итоговый Volume Score.
func Score(s *VolumeStats) float64 {
	if s == nil {
		return 0
	}
	return s.Score
}

func calcVWAP(candles []api.Candle) float64 {
	tv, vol := 0.0, 0.0
	for _, c := range candles {
		h, _ := c.High.Float64()
		l, _ := c.Low.Float64()
		cl, _ := c.Close.Float64()
		v, _ := c.Volume.Float64()
		tv += (h + l + cl) / 3 * v
		vol += v
	}
	if vol == 0 {
		return 0
	}
	return tv / vol
}

func clampV(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
