// Пакет scanners — сканирование и ранжирование рынка.
// ranker.go — ранжирование по ОТНОСИТЕЛЬНЫМ метрикам, а не абсолютным.
//
// Ключевое отличие:
//   Старая логика: Volume = объём в млн USD → BTC/ETH всегда топ
//   Новая логика:  Volume = текущий / средний → монета с 5× спайком обгоняет BTC
//
//   Старая логика: OI = абсолютное значение в USD
//   Новая логика:  OI = отклонение от исторического среднего (+/-)
package scanners

import (
	"math"
	"sort"

	"bybit-elite-signal/api"
	"github.com/shopspring/decimal"
)

// SymbolScore — многофакторный рейтинг одного символа.
type SymbolScore struct {
	Symbol            string
	VolumeScore       float64 // относительный объём (current/avg_history)
	RelVolume         float64 // насыщённый относительный объём
	LiquidityScore    float64 // ликвидность стакана
	OIScore           float64 // относительный OI (отклонение от среднего)
	VolatilityScore   float64 // волатильность 24ч
	TrendScore        float64 // позиция в 24ч диапазоне
	InstitScore       float64 // институциональная активность
	MomentumScore     float64 // импульс
	MarketOpportunity float64 // итоговый балл
}

// RankSymbols ранжирует тикеры и возвращает топ-N по MarketOpportunity.
// baseline содержит исторические средние для относительных расчётов.
func RankSymbols(tickers []api.Ticker, topN int, baseline *Baseline) []SymbolScore {
	scores := make([]SymbolScore, 0, len(tickers))
	for _, t := range tickers {
		scores = append(scores, scoreSymbol(t, baseline))
	}
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].MarketOpportunity > scores[j].MarketOpportunity
	})
	if topN > 0 && topN < len(scores) {
		return scores[:topN]
	}
	return scores
}

func scoreSymbol(t api.Ticker, bl *Baseline) SymbolScore {
	s := SymbolScore{Symbol: t.Symbol}

	vol24hUSD, _ := t.Turnover24h.Float64() // объём в USDT (одинаковая база для всех)
	oiUSD, _ := t.OpenInterestVal.Float64()

	// ── Объём: относительный (current / historical_avg) ──────────────────
	if bl != nil && bl.HasHistory(t.Symbol) {
		// Есть история — считаем относительный объём
		// 1.0 = обычный объём, 3.0 = втрое выше нормы
		relVol := bl.RelativeVolume(t.Symbol, vol24hUSD)
		// Насыщение: 5× и выше = 100 баллов
		s.VolumeScore = clamp(relVol/5.0*100, 0, 100)
	} else {
		// Нет истории — логарифмическая нормализация (сглаживает BTC vs altcoin разрыв)
		s.VolumeScore = clamp(logScale(vol24hUSD, 1e6, 5e9), 0, 100)
	}
	s.RelVolume = s.VolumeScore

	// ── OI: относительный (отклонение от среднего) ───────────────────────
	if bl != nil && bl.HasHistory(t.Symbol) {
		relOI := bl.RelativeOI(t.Symbol, oiUSD)
		// +1.0 (100% выше среднего) = 100 баллов; 0 = 50 баллов; -0.5 = 0 баллов
		s.OIScore = clamp((relOI+0.5)/1.5*100, 0, 100)
	} else {
		s.OIScore = clamp(logScale(oiUSD, 1e5, 1e9), 0, 100)
	}

	// ── Ликвидность: тем выше, чем меньше спред ──────────────────────────
	if !t.Ask1Price.IsZero() && !t.Bid1Price.IsZero() {
		spread := t.Ask1Price.Sub(t.Bid1Price)
		spreadPct, _ := spread.Div(t.LastPrice).Mul(decimal.NewFromInt(100)).Float64()
		s.LiquidityScore = clamp(100-spreadPct*1000, 0, 100)
	}

	// ── Волатильность: изменение цены за 24ч ─────────────────────────────
	pct24h, _ := t.Price24hPct.Abs().Float64()
	s.VolatilityScore = clamp(pct24h*20, 0, 100) // 5% = 100

	// ── Тренд: позиция цены в 24ч диапазоне ──────────────────────────────
	high, _ := t.High24h.Float64()
	low, _ := t.Low24h.Float64()
	last, _ := t.LastPrice.Float64()
	if high != low {
		s.TrendScore = clamp((last-low)/(high-low)*100, 0, 100)
	}

	// ── Институциональный интерес: OI + Volume совместно ─────────────────
	s.InstitScore = clamp((s.OIScore+s.VolumeScore)/2, 0, 100)

	// ── Импульс ───────────────────────────────────────────────────────────
	s.MomentumScore = clamp(pct24h*15, 0, 100)

	// ── Итоговый балл ─────────────────────────────────────────────────────
	// Относительный объём наиболее важен: аномальная активность = потенциал движения
	s.MarketOpportunity =
		s.VolumeScore*0.30 +
		s.OIScore*0.25 +
		s.LiquidityScore*0.15 +
		s.VolatilityScore*0.15 +
		s.TrendScore*0.10 +
		s.MomentumScore*0.05

	return s
}

// logScale нормализует значение по логарифмической шкале → 0..100.
// Сглаживает огромный разрыв между BTC ($50B OI) и altcoin ($50M OI).
func logScale(val, minVal, maxVal float64) float64 {
	if val <= 0 || minVal <= 0 || maxVal <= minVal {
		return 0
	}
	logVal := math.Log10(val)
	logMin := math.Log10(minVal)
	logMax := math.Log10(maxVal)
	if logMax <= logMin {
		return 0
	}
	return (logVal - logMin) / (logMax - logMin) * 100
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
