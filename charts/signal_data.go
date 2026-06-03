// Пакет charts — генерация торговых графиков.
// signal_data.go — структура входных данных сигнала для визуализации.
package charts

import signalengine "bybit-elite-signal/signal_engine"

// SignalChartData — все данные необходимые для отрисовки графика.
type SignalChartData struct {
	Symbol    string
	Direction string // "Long" | "Short"
	Timeframe string // "15m", "1h" и т.д.

	Price float64 // текущая цена

	// Зона входа
	EntryZoneLow  float64
	EntryZoneHigh float64
	EntryIdeal    float64

	StopLoss   float64
	TakeProfit float64
	TPs        []float64 // все уровни TP (если несколько)
	RR         float64

	// Order Block
	OBLow  float64
	OBHigh float64

	// FVG
	FVGLow  float64
	FVGHigh float64

	// S/R уровни
	SRLevels []float64

	// Канал
	ChannelLow       float64
	ChannelHigh      float64
	ChannelDirection string // "up" | "down" | "flat"

	// Мета
	Regime         string
	Session        string
	Score          float64
	Catalyst       string
	EdgeFactors    []string
	MoveNumber     int
	PathScore      float64
	LiquidityBias  float64
	Deadline       string
	Mode           string // "⚡ INTRADAY" | "📊 SWING"
}

// FromSignal конвертирует Signal в SignalChartData.
func FromSignal(sig *signalengine.Signal, tps []float64, mode, deadline string, cfg *ChartConfig) *SignalChartData {
	d := &SignalChartData{
		Symbol:        sig.Symbol,
		Direction:     sig.Direction,
		Timeframe:     "15m",
		Price:         sig.MarkPrice,
		EntryZoneLow:  sig.Entry * 0.999,
		EntryZoneHigh: sig.Entry * 1.001,
		EntryIdeal:    sig.Entry,
		StopLoss:      sig.StopLoss,
		TakeProfit:    sig.TakeProfit,
		TPs:           tps,
		RR:            sig.RR,
		Score:         sig.Score,
		Catalyst:      sig.Setup,
		Deadline:      deadline,
		Mode:          mode,
	}

	// Рассчитываем OB зону из Entry (Entry = верхняя граница OB для Long)
	riskDist := abs64(sig.Entry - sig.StopLoss)
	if sig.Direction == "Long" {
		d.OBHigh = sig.Entry
		d.OBLow = sig.Entry - riskDist*0.5
		d.FVGLow = sig.Entry
		d.FVGHigh = sig.Entry + riskDist*0.8
	} else {
		d.OBLow = sig.Entry
		d.OBHigh = sig.Entry + riskDist*0.5
		d.FVGHigh = sig.Entry
		d.FVGLow = sig.Entry - riskDist*0.8
	}

	// Канал
	d.ChannelLow = sig.StopLoss * 0.998
	d.ChannelHigh = sig.TakeProfit * 1.002
	if sig.Direction == "Long" {
		d.ChannelDirection = "up"
	} else {
		d.ChannelDirection = "down"
	}

	// S/R уровни: SL, Entry, TP
	d.SRLevels = []float64{sig.StopLoss, sig.Entry, sig.TakeProfit}
	for _, tp := range tps {
		d.SRLevels = append(d.SRLevels, tp)
	}

	return d
}

func abs64(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
