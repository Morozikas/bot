// Пакет footprint — анализ ордер-флоу.
// cvd.go — Cumulative Volume Delta: суммарная дельта за период.
package footprint

// CVDTrend — тренд CVD.
type CVDTrend string

const (
	CVDBullish CVDTrend = "Bullish"
	CVDBearish CVDTrend = "Bearish"
	CVDNeutral CVDTrend = "Neutral"
)

// CVDAnalysis — анализ накопленной дельты.
type CVDAnalysis struct {
	Total    float64
	Trend    CVDTrend
	Diverges bool // дивергенция с ценой
}

// AnalyzeCVD вычисляет тренд CVD.
func AnalyzeCVD(dc []DeltaCandle) *CVDAnalysis {
	r := &CVDAnalysis{}
	if len(dc) == 0 {
		return r
	}
	r.Total = dc[len(dc)-1].CumDelta
	if r.Total > 0 {
		r.Trend = CVDBullish
	} else if r.Total < 0 {
		r.Trend = CVDBearish
	} else {
		r.Trend = CVDNeutral
	}
	r.Diverges = DeltaDivergence(dc)
	return r
}
