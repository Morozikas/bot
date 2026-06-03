// Пакет structure — анализ рыночной структуры.
// trend.go — определение режима рынка: тренд, диапазон, компрессия, экспансия.
package structure

import (
	"math"

	"bybit-elite-signal/api"
)

// MarketRegime — режим рынка.
type MarketRegime string

const (
	RegimeBullish     MarketRegime = "Bullish"
	RegimeBearish     MarketRegime = "Bearish"
	RegimeRange       MarketRegime = "Range"
	RegimeBOS         MarketRegime = "BOS"
	RegimeCHOCH       MarketRegime = "CHOCH"
	RegimeExpansion   MarketRegime = "Expansion"
	RegimeCompression MarketRegime = "Compression"
)

// TrendResult — полный результат анализа тренда.
type TrendResult struct {
	Regime       MarketRegime
	Highs        []SwingPoint
	Lows         []SwingPoint
	BOS          *BOSResult
	CHOCH        *CHOCHResult
	MSS          *MSSResult
	ATR14        float64
	ATR5         float64
	IsCompressed bool
	IsExpanding  bool
}

// AnalyzeTrend выполняет полный анализ тренда.
func AnalyzeTrend(symbol, tf string, candles []api.Candle, lookback int) *TrendResult {
	r := &TrendResult{}
	if len(candles) < 15 {
		return r
	}
	if lookback > 0 && lookback < len(candles) {
		candles = candles[len(candles)-lookback:]
	}

	r.Highs, r.Lows = DetectSwings(candles, 5)
	r.BOS = DetectBOS(candles, r.Highs, r.Lows)
	r.CHOCH = DetectCHOCH(candles, r.Highs, r.Lows)
	r.MSS = DetectMSS(candles, r.Highs, r.Lows)

	r.ATR14 = atrTrend(candles, 14)
	r.ATR5 = atrTrend(candles, 5)

	if r.ATR14 > 0 {
		ratio := r.ATR5 / r.ATR14
		r.IsCompressed = ratio < 0.6
		r.IsExpanding = ratio > 1.4
	}

	switch {
	case r.CHOCH.Detected:
		r.Regime = RegimeCHOCH
	case r.BOS.Detected:
		r.Regime = RegimeBOS
	case r.IsExpanding:
		r.Regime = RegimeExpansion
	case r.IsCompressed:
		r.Regime = RegimeCompression
	default:
		r.Regime = classifyTrend(r.Highs, r.Lows)
	}
	return r
}

func classifyTrend(highs, lows []SwingPoint) MarketRegime {
	if len(highs) < 2 || len(lows) < 2 {
		return RegimeRange
	}
	hh := highs[len(highs)-1].Price > highs[len(highs)-2].Price
	hl := lows[len(lows)-1].Price > lows[len(lows)-2].Price
	if hh && hl {
		return RegimeBullish
	}
	lh := highs[len(highs)-1].Price < highs[len(highs)-2].Price
	ll := lows[len(lows)-1].Price < lows[len(lows)-2].Price
	if lh && ll {
		return RegimeBearish
	}
	return RegimeRange
}

func atrTrend(candles []api.Candle, n int) float64 {
	if len(candles) < n+1 {
		return 0
	}
	src := candles[len(candles)-n-1:]
	total := 0.0
	for i := 1; i <= n; i++ {
		h, _ := src[i].High.Float64()
		l, _ := src[i].Low.Float64()
		pc, _ := src[i-1].Close.Float64()
		tr := math.Max(h-l, math.Max(math.Abs(h-pc), math.Abs(l-pc)))
		total += tr
	}
	return total / float64(n)
}
