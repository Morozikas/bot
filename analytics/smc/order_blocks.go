// Пакет smc — Smart Money Concepts.
// order_blocks.go — Order Blocks: бычьи и медвежьи блоки институциональных ордеров.
package smc

import "bybit-elite-signal/api"

// OBType — тип ордер-блока.
type OBType string

const (
	BullishOB    OBType = "BullishOB"
	BearishOB    OBType = "BearishOB"
)

// OrderBlock — обнаруженный ордер-блок.
type OrderBlock struct {
	Type             OBType
	Top              float64
	Bottom           float64
	Volume           float64
	RetestCount      int
	Freshness        float64 // 1.0 = нетронутый
	ReactionStrength float64
	VolumeStrength   float64
	Age              int
	Quality          float64
	CandleIdx        int
}

// DetectOrderBlocks находит ордер-блоки.
func DetectOrderBlocks(candles []api.Candle, maxAge int) []OrderBlock {
	result := make([]OrderBlock, 0)
	n := len(candles)
	if n < 5 {
		return result
	}
	for i := 1; i < n-3; i++ {
		age := n - 1 - i
		if maxAge > 0 && age > maxAge {
			continue
		}
		o, _ := candles[i].Open.Float64()
		cl, _ := candles[i].Close.Float64()
		h, _ := candles[i].High.Float64()
		l, _ := candles[i].Low.Float64()
		v, _ := candles[i].Volume.Float64()
		nxt, _ := candles[i+1].Close.Float64()

		if cl < o && (nxt-cl)/cl > 0.003 {
			ob := OrderBlock{Type: BullishOB, Top: h, Bottom: l, Volume: v, Age: age, CandleIdx: i}
			ob.RetestCount = obRetests(candles[i+1:], l, h, BullishOB)
			ob.Freshness = obFreshness(candles[i+1:], l, h, BullishOB)
			ob.ReactionStrength = (nxt - cl) / cl * 100
			ob.VolumeStrength = obVolStrength(candles, i)
			ob.Quality = obQuality(ob)
			result = append(result, ob)
		}
		if cl > o && (cl-nxt)/cl > 0.003 {
			ob := OrderBlock{Type: BearishOB, Top: h, Bottom: l, Volume: v, Age: age, CandleIdx: i}
			ob.RetestCount = obRetests(candles[i+1:], l, h, BearishOB)
			ob.Freshness = obFreshness(candles[i+1:], l, h, BearishOB)
			ob.ReactionStrength = (cl - nxt) / cl * 100
			ob.VolumeStrength = obVolStrength(candles, i)
			ob.Quality = obQuality(ob)
			result = append(result, ob)
		}
	}
	return result
}

// BullishOBs фильтрует бычьи ордер-блоки.
func BullishOBs(candles []api.Candle, maxAge int) []OrderBlock {
	all := DetectOrderBlocks(candles, maxAge)
	result := make([]OrderBlock, 0)
	for _, o := range all {
		if o.Type == BullishOB {
			result = append(result, o)
		}
	}
	return result
}

// BearishOBs фильтрует медвежьи ордер-блоки.
func BearishOBs(candles []api.Candle, maxAge int) []OrderBlock {
	all := DetectOrderBlocks(candles, maxAge)
	result := make([]OrderBlock, 0)
	for _, o := range all {
		if o.Type == BearishOB {
			result = append(result, o)
		}
	}
	return result
}

// BestOB возвращает OB с наибольшим Quality.
func BestOB(obs []OrderBlock) *OrderBlock {
	if len(obs) == 0 {
		return nil
	}
	best := &obs[0]
	for i := range obs {
		if obs[i].Quality > best.Quality {
			best = &obs[i]
		}
	}
	return best
}

func obRetests(candles []api.Candle, bottom, top float64, t OBType) int {
	count := 0
	for _, c := range candles {
		h, _ := c.High.Float64()
		l, _ := c.Low.Float64()
		if t == BullishOB && l <= top && h >= bottom {
			count++
		}
		if t == BearishOB && h >= bottom && l <= top {
			count++
		}
	}
	return count
}

func obFreshness(candles []api.Candle, bottom, top float64, t OBType) float64 {
	mid := (top + bottom) / 2
	for _, c := range candles {
		cl, _ := c.Close.Float64()
		h, _ := c.High.Float64()
		l, _ := c.Low.Float64()
		if t == BullishOB && cl < mid {
			return 0.0
		}
		if t == BearishOB && cl > mid {
			return 0.0
		}
		if t == BullishOB && l < mid {
			return 0.5
		}
		if t == BearishOB && h > mid {
			return 0.5
		}
	}
	return 1.0
}

func obVolStrength(candles []api.Candle, idx int) float64 {
	start := idx - 20
	if start < 0 {
		start = 0
	}
	avg := 0.0
	for _, c := range candles[start:idx] {
		v, _ := c.Volume.Float64()
		avg += v
	}
	if idx-start == 0 {
		return 50
	}
	avg /= float64(idx - start)
	cur, _ := candles[idx].Volume.Float64()
	if avg <= 0 {
		return 50
	}
	ratio := cur / avg / 3 * 100
	if ratio > 100 {
		return 100
	}
	return ratio
}

func obQuality(ob OrderBlock) float64 {
	score := 40.0 + ob.Freshness*30
	if ob.ReactionStrength >= 1.0 {
		score += 15
	} else if ob.ReactionStrength >= 0.5 {
		score += 8
	}
	score += ob.VolumeStrength * 0.1
	switch {
	case ob.RetestCount == 0:
		score += 10
	case ob.RetestCount == 1:
		score += 5
	case ob.RetestCount >= 3:
		score -= 10
	}
	if ob.Age > 50 {
		score -= 10
	} else if ob.Age > 20 {
		score -= 5
	}
	if score > 100 {
		return 100
	}
	if score < 0 {
		return 0
	}
	return score
}
