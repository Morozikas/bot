// Пакет correlations — анализ корреляций.
// btc.go — корреляция с BTC/ETH: поддерживают ли макро-тренды направление сигнала.
package correlations

import "bybit-elite-signal/api"

// CorrResult — результат проверки макро-корреляций.
type CorrResult struct {
	BTCTrend string  // "Bullish" | "Bearish" | "Neutral"
	ETHTrend string
	ScoreAdj float64 // +5 если согласуются, -10 если противоречат
	Against  bool
}

// AnalyzeBTCETH проверяет тренды BTC и ETH против направления сигнала.
func AnalyzeBTCETH(btcCandles, ethCandles []api.Candle, signalDirection string) *CorrResult {
	r := &CorrResult{}
	r.BTCTrend = trendEMA(btcCandles)
	r.ETHTrend = trendEMA(ethCandles)
	bullish := r.BTCTrend == "Bullish" && r.ETHTrend == "Bullish"
	bearish := r.BTCTrend == "Bearish" && r.ETHTrend == "Bearish"
	switch signalDirection {
	case "Long":
		if bullish {
			r.ScoreAdj = 5
		} else if bearish {
			r.ScoreAdj = -10
			r.Against = true
		}
	case "Short":
		if bearish {
			r.ScoreAdj = 5
		} else if bullish {
			r.ScoreAdj = -10
			r.Against = true
		}
	}
	return r
}

func trendEMA(candles []api.Candle) string {
	if len(candles) < 21 {
		return "Neutral"
	}
	e9 := ema(candles, 9)
	e21 := ema(candles, 21)
	if e9 > e21 {
		return "Bullish"
	}
	if e9 < e21 {
		return "Bearish"
	}
	return "Neutral"
}

func ema(candles []api.Candle, period int) float64 {
	if len(candles) < period {
		return 0
	}
	k := 2.0 / float64(period+1)
	v, _ := candles[0].Close.Float64()
	for i := 1; i < len(candles); i++ {
		c, _ := candles[i].Close.Float64()
		v = c*k + v*(1-k)
	}
	return v
}
