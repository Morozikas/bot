// Пакет smc — Smart Money Concepts.
// premium_discount.go — Premium / Discount Zone и Balanced Price Range.
package smc

// PDZone — Premium или Discount зона.
type PDZone string

const (
	PremiumZone  PDZone = "Premium"
	DiscountZone PDZone = "Discount"
	FairValueZone PDZone = "FairValue"
)

// ClassifyZone определяет, в какой зоне находится текущая цена.
// swingHigh, swingLow — крайние точки последнего движения.
func ClassifyZone(price, swingHigh, swingLow float64) PDZone {
	if swingHigh <= swingLow {
		return FairValueZone
	}
	pct := (price - swingLow) / (swingHigh - swingLow) * 100
	if pct >= 62.5 {
		return PremiumZone
	}
	if pct <= 37.5 {
		return DiscountZone
	}
	return FairValueZone
}

// BPR (Balanced Price Range) — зона между двумя перекрывающимися FVG.
type BPR struct {
	Top    float64
	Bottom float64
}

// FindBPR ищет Balanced Price Range между двумя FVG.
func FindBPR(fvg1, fvg2 FVG) *BPR {
	// BPR существует когда бычий и медвежий FVG перекрываются
	if fvg1.Type == BullishFVG && fvg2.Type == BearishFVG {
		top := min2(fvg1.Top, fvg2.Top)
		bottom := max2(fvg1.Bottom, fvg2.Bottom)
		if top > bottom {
			return &BPR{Top: top, Bottom: bottom}
		}
	}
	return nil
}

func min2(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max2(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
