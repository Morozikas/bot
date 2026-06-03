// Пакет backtesting — исторический бэктест сигналов на OHLCV-данных.
// backtester.go — симуляция входов по entry/SL/TP на свечах с расчётом полной статистики.
package backtesting

import (
	"fmt"
	"math"
	"time"

	"bybit-elite-signal/api"
	"bybit-elite-signal/config"
)

// Result — итог бэктеста.
type Result struct {
	Symbol       string
	TotalTrades  int
	Wins         int
	Losses       int
	WinRate      float64
	ProfitFactor float64
	Expectancy   float64
	MaxDrawdown  float64
	NetPnLPct    float64
	StartEquity  float64
	EndEquity    float64
	StartDate    time.Time
	EndDate      time.Time
}

// TradeEntry — входные данные одной симулированной сделки.
type TradeEntry struct {
	Entry     float64
	SL        float64
	TP        float64
	RR        float64
	Direction string
}

// Backtester симулирует производительность сигналов на исторических данных.
type Backtester struct {
	cfg *config.SignalConfig
}

// NewBacktester создаёт Backtester.
func NewBacktester(cfg *config.SignalConfig) *Backtester {
	return &Backtester{cfg: cfg}
}

// Run запускает бэктест по серии свечей и списку входов.
func (b *Backtester) Run(symbol string, candles []api.Candle, entries []TradeEntry) *Result {
	r := &Result{Symbol: symbol, StartEquity: 10000}
	if len(candles) == 0 || len(entries) == 0 {
		return r
	}
	r.StartDate = time.UnixMilli(candles[0].StartTime)
	r.EndDate = time.UnixMilli(candles[len(candles)-1].StartTime)

	equity, peak := r.StartEquity, r.StartEquity
	gains, losses := 0.0, 0.0

	for _, e := range entries {
		outcome, pnlPct := simulate(e, candles)
		equity *= 1 + pnlPct/100
		if equity > peak {
			peak = equity
		}
		if dd := (peak - equity) / peak * 100; dd > r.MaxDrawdown {
			r.MaxDrawdown = dd
		}
		r.TotalTrades++
		if outcome == "Win" {
			r.Wins++
			gains += pnlPct
		} else {
			r.Losses++
			losses += math.Abs(pnlPct)
		}
	}

	r.EndEquity = equity
	r.NetPnLPct = (equity - r.StartEquity) / r.StartEquity * 100
	if r.TotalTrades > 0 {
		r.WinRate = float64(r.Wins) / float64(r.TotalTrades) * 100
	}
	if losses > 0 {
		r.ProfitFactor = gains / losses
	}
	if r.TotalTrades > 0 {
		r.Expectancy = (gains - losses) / float64(r.TotalTrades)
	}
	return r
}

const riskPct = 1.0 // % риска на сделку в бэктесте

func simulate(e TradeEntry, candles []api.Candle) (string, float64) {
	riskDist := math.Abs(e.Entry - e.SL)
	rewardDist := math.Abs(e.Entry - e.TP)
	if riskDist <= 0 {
		return "Loss", -riskPct
	}
	for _, c := range candles {
		h, _ := c.High.Float64()
		l, _ := c.Low.Float64()
		if e.Direction == "Long" {
			if l <= e.SL {
				return "Loss", -riskPct
			}
			if h >= e.TP {
				return "Win", rewardDist / riskDist * riskPct
			}
		} else {
			if h >= e.SL {
				return "Loss", -riskPct
			}
			if l <= e.TP {
				return "Win", rewardDist / riskDist * riskPct
			}
		}
	}
	return "Loss", -riskPct / 2
}

// Print возвращает отформатированный отчёт о бэктесте.
func (r *Result) Print() string {
	return fmt.Sprintf(
		"БЭКТЕСТ %s (%s — %s)\n"+
			"Сделок: %d | Побед: %d (%.1f%%) | Потерь: %d\n"+
			"PF: %.2f | Expectancy: %.2f%% | MaxDD: %.2f%%\n"+
			"Итог P&L: %.2f%% | Капитал: $%.2f → $%.2f",
		r.Symbol,
		r.StartDate.Format("2006-01-02"),
		r.EndDate.Format("2006-01-02"),
		r.TotalTrades, r.Wins, r.WinRate, r.Losses,
		r.ProfitFactor, r.Expectancy, r.MaxDrawdown,
		r.NetPnLPct, r.StartEquity, r.EndEquity,
	)
}
