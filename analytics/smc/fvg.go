// Пакет smc — Smart Money Concepts.
// fvg.go — Fair Value Gap: бычьи, медвежьи и инверсионные зоны дисбаланса.
package smc

import "bybit-elite-signal/api"

// FVGType — тип Fair Value Gap.
type FVGType string

const (
	BullishFVG  FVGType = "BullishFVG"
	BearishFVG  FVGType = "BearishFVG"
	InvFVG      FVGType = "InversionFVG"
)

// FVG — обнаруженная зона FVG.
type FVG struct {
	Type        FVGType
	Top         float64
	Bottom      float64
	Size        float64
	SizePct     float64
	Age         int
	RetestCount int
	IsFilled    bool
	Quality     float64
	CandleIdx   int
}

// DetectFVG находит все незакрытые FVG в серии свечей.
func DetectFVG(candles []api.Candle, maxAge int) []FVG {
	result := make([]FVG, 0)
	n := len(candles)
	if n < 3 {
		return result
	}
	for i := 1; i < n-1; i++ {
		prevH, _ := candles[i-1].High.Float64()
		prevL, _ := candles[i-1].Low.Float64()
		nextH, _ := candles[i+1].High.Float64()
		nextL, _ := candles[i+1].Low.Float64()
		mid, _ := candles[i].Close.Float64()
		age := n - 1 - i
		if maxAge > 0 && age > maxAge {
			continue
		}
		if mid <= 0 {
			continue
		}
		if nextL > prevH {
			fvg := FVG{Type: BullishFVG, Top: nextL, Bottom: prevH,
				Size: nextL - prevH, SizePct: (nextL - prevH) / mid * 100,
				Age: age, CandleIdx: i}
			fvg.RetestCount = countFVGRetests(candles[i+1:], fvg.Bottom, fvg.Top, BullishFVG)
			fvg.IsFilled = fvgFilled(candles[i+1:], fvg.Bottom, BullishFVG)
			if !fvg.IsFilled {
				fvg.Quality = fvgQuality(fvg)
				result = append(result, fvg)
			}
		}
		if nextH < prevL {
			fvg := FVG{Type: BearishFVG, Top: prevL, Bottom: nextH,
				Size: prevL - nextH, SizePct: (prevL - nextH) / mid * 100,
				Age: age, CandleIdx: i}
			fvg.RetestCount = countFVGRetests(candles[i+1:], fvg.Bottom, fvg.Top, BearishFVG)
			fvg.IsFilled = fvgFilled(candles[i+1:], fvg.Top, BearishFVG)
			if !fvg.IsFilled {
				fvg.Quality = fvgQuality(fvg)
				result = append(result, fvg)
			}
		}
	}
	return result
}

// BullishFVGs возвращает только бычьи FVG.
func BullishFVGs(candles []api.Candle, maxAge int) []FVG {
	all := DetectFVG(candles, maxAge)
	result := make([]FVG, 0)
	for _, f := range all {
		if f.Type == BullishFVG {
			result = append(result, f)
		}
	}
	return result
}

// BearishFVGs возвращает только медвежьи FVG.
func BearishFVGs(candles []api.Candle, maxAge int) []FVG {
	all := DetectFVG(candles, maxAge)
	result := make([]FVG, 0)
	for _, f := range all {
		if f.Type == BearishFVG {
			result = append(result, f)
		}
	}
	return result
}

// BestFVG возвращает FVG с максимальным Quality.
func BestFVG(fvgs []FVG) *FVG {
	if len(fvgs) == 0 {
		return nil
	}
	best := &fvgs[0]
	for i := range fvgs {
		if fvgs[i].Quality > best.Quality {
			best = &fvgs[i]
		}
	}
	return best
}

func countFVGRetests(candles []api.Candle, bottom, top float64, t FVGType) int {
	count := 0
	for _, c := range candles {
		h, _ := c.High.Float64()
		l, _ := c.Low.Float64()
		if t == BullishFVG && l <= top && l >= bottom {
			count++
		}
		if t == BearishFVG && h >= bottom && h <= top {
			count++
		}
	}
	return count
}

func fvgFilled(candles []api.Candle, threshold float64, t FVGType) bool {
	for _, c := range candles {
		h, _ := c.High.Float64()
		l, _ := c.Low.Float64()
		if t == BullishFVG && l < threshold {
			return true
		}
		if t == BearishFVG && h > threshold {
			return true
		}
	}
	return false
}

func fvgQuality(fvg FVG) float64 {
	score := 50.0
	if fvg.SizePct >= 0.1 {
		score += 15
	} else if fvg.SizePct >= 0.05 {
		score += 8
	}
	if fvg.Age <= 3 {
		score += 20
	} else if fvg.Age <= 10 {
		score += 10
	} else if fvg.Age > 30 {
		score -= 10
	}
	if fvg.RetestCount == 1 {
		score += 10
	} else if fvg.RetestCount >= 3 {
		score -= 15
	}
	if score > 100 {
		return 100
	}
	if score < 0 {
		return 0
	}
	return score
}
