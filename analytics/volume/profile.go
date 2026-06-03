// Пакет volume — анализ объёма.
// profile.go — построение Volume Profile: распределение объёма по ценовым уровням.
package volume

import (
	"math"
	"sort"

	"bybit-elite-signal/api"
)

// Profile — итоговый Volume Profile.
type Profile struct {
	POC    float64   // Point of Control
	VAH    float64   // Value Area High (70%)
	VAL    float64   // Value Area Low
	HVNs   []float64 // High Volume Nodes
	LVNs   []float64 // Low Volume Nodes
	Levels []Level
}

// Level — один ценовой бакет профиля.
type Level struct {
	Price  float64
	Volume float64
	IsHVN  bool
	IsLVN  bool
	IsPOC  bool
}

// Build строит Volume Profile из свечей.
func Build(candles []api.Candle, numBuckets int) *Profile {
	if len(candles) == 0 || numBuckets <= 0 {
		return &Profile{}
	}
	priceHigh, priceLow := 0.0, math.MaxFloat64
	for _, c := range candles {
		h, _ := c.High.Float64()
		l, _ := c.Low.Float64()
		if h > priceHigh {
			priceHigh = h
		}
		if l < priceLow {
			priceLow = l
		}
	}
	if priceHigh <= priceLow {
		return &Profile{}
	}
	bSize := (priceHigh - priceLow) / float64(numBuckets)
	buckets := make([]float64, numBuckets)
	for _, c := range candles {
		h, _ := c.High.Float64()
		l, _ := c.Low.Float64()
		v, _ := c.Volume.Float64()
		lo := int((l - priceLow) / bSize)
		hi := int((h - priceLow) / bSize)
		if lo < 0 {
			lo = 0
		}
		if hi >= numBuckets {
			hi = numBuckets - 1
		}
		span := hi - lo + 1
		for b := lo; b <= hi; b++ {
			buckets[b] += v / float64(span)
		}
	}
	levels := make([]Level, numBuckets)
	maxVol, totalVol := 0.0, 0.0
	pocIdx := 0
	for i := range buckets {
		levels[i] = Level{Price: priceLow + float64(i)*bSize + bSize/2, Volume: buckets[i]}
		totalVol += buckets[i]
		if buckets[i] > maxVol {
			maxVol = buckets[i]
			pocIdx = i
		}
	}
	p := &Profile{POC: levels[pocIdx].Price}
	levels[pocIdx].IsPOC = true

	// Value Area (70%)
	vaTarget := totalVol * 0.70
	vaVol := levels[pocIdx].Volume
	lo, hi := pocIdx, pocIdx
	for vaVol < vaTarget && (lo > 0 || hi < numBuckets-1) {
		loV, hiV := 0.0, 0.0
		if lo > 0 {
			loV = buckets[lo-1]
		}
		if hi < numBuckets-1 {
			hiV = buckets[hi+1]
		}
		if loV >= hiV && lo > 0 {
			lo--
			vaVol += levels[lo].Volume
		} else if hi < numBuckets-1 {
			hi++
			vaVol += levels[hi].Volume
		} else {
			break
		}
	}
	p.VAL = levels[lo].Price
	p.VAH = levels[hi].Price

	avg := totalVol / float64(numBuckets)
	for i := range levels {
		if levels[i].Volume >= avg*1.5 {
			levels[i].IsHVN = true
			p.HVNs = append(p.HVNs, levels[i].Price)
		} else if levels[i].Volume <= avg*0.5 {
			levels[i].IsLVN = true
			p.LVNs = append(p.LVNs, levels[i].Price)
		}
	}
	p.Levels = levels
	sort.Slice(p.HVNs, func(i, j int) bool { return p.HVNs[i] > p.HVNs[j] })
	return p
}
