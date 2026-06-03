// Пакет derivatives — аналитика деривативов.
// ls_ratio.go — соотношение лонг/шорт позиций и его интерпретация.
package derivatives

import "bybit-elite-signal/api"

// LSRatioResult — результат анализа L/S ratio.
type LSRatioResult struct {
	BuyRatio   float64
	SellRatio  float64
	LSRatio    float64 // BuyRatio / SellRatio
	Sentiment  string  // "Bullish" | "Bearish" | "Neutral"
	ScoreAdj   float64
}

// AnalyzeLSRatio интерпретирует соотношение лонг/шорт.
// Парадоксально: высокое соотношение лонгов = перегрев = потенциально медвежий сигнал.
func AnalyzeLSRatio(history []api.LongShortRatio) *LSRatioResult {
	r := &LSRatioResult{}
	if len(history) == 0 {
		return r
	}
	buy, _ := history[0].BuyRatio.Float64()
	sell, _ := history[0].SellRatio.Float64()
	r.BuyRatio = buy
	r.SellRatio = sell
	if sell > 0 {
		r.LSRatio = buy / sell
	}
	switch {
	case r.LSRatio > 2.0:
		r.Sentiment = "Bearish" // слишком много лонгов = риск падения
		r.ScoreAdj = -5
	case r.LSRatio < 0.5:
		r.Sentiment = "Bullish" // слишком много шортов = риск шорт-сквиза
		r.ScoreAdj = 5
	default:
		r.Sentiment = "Neutral"
	}
	return r
}
