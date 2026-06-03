// Пакет statistics — статистика торговли.
// setup.go — скользящая история из 500 сигналов: WR, PF, Expectancy, Sharpe, Sortino, MaxDD.
package statistics

import (
	"math"
	"sync"
	"time"
)

// Record — запись одного исторического сигнала.
type Record struct {
	ID        string
	Symbol    string
	Direction string
	Entry     float64
	SL        float64
	TP        float64
	RR        float64
	Score     float64
	CreatedAt time.Time
	ClosedAt  time.Time
	Outcome   string  // "Win" | "Loss" | "Pending"
	PnLPct    float64
}

// Performance — сводная статистика.
type Performance struct {
	TotalSignals int
	Wins         int
	Losses       int
	WinRate      float64
	ProfitFactor float64
	Expectancy   float64
	SharpeRatio  float64
	SortinoRatio float64
	MaxDrawdown  float64
	AvgRR        float64
}

// History хранит скользящее окно из maxSize сигналов.
type History struct {
	records []Record
	mu      sync.RWMutex
	maxSize int
}

// NewHistory создаёт History.
func NewHistory(maxSize int) *History {
	if maxSize <= 0 {
		maxSize = 500
	}
	return &History{maxSize: maxSize}
}

// Add добавляет запись.
func (h *History) Add(r Record) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r)
	if len(h.records) > h.maxSize {
		h.records = h.records[len(h.records)-h.maxSize:]
	}
}

// SetOutcome фиксирует результат сигнала.
func (h *History) SetOutcome(id, outcome string, pnlPct float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := range h.records {
		if h.records[i].ID == id {
			h.records[i].Outcome = outcome
			h.records[i].PnLPct = pnlPct
			h.records[i].ClosedAt = time.Now()
			return
		}
	}
}

// Stats вычисляет статистику по последним n закрытым записям.
func (h *History) Stats(n int) Performance {
	h.mu.RLock()
	defer h.mu.RUnlock()

	closed := make([]Record, 0)
	for _, r := range h.records {
		if r.Outcome == "Win" || r.Outcome == "Loss" {
			closed = append(closed, r)
		}
	}
	if n > 0 && len(closed) > n {
		closed = closed[len(closed)-n:]
	}
	p := Performance{TotalSignals: len(closed)}
	if len(closed) == 0 {
		return p
	}

	gains, losses, rrSum := 0.0, 0.0, 0.0
	for _, r := range closed {
		if r.Outcome == "Win" {
			p.Wins++
			gains += r.PnLPct
		} else {
			p.Losses++
			losses += math.Abs(r.PnLPct)
		}
		rrSum += r.RR
	}
	p.WinRate = float64(p.Wins) / float64(len(closed)) * 100
	p.AvgRR = rrSum / float64(len(closed))
	if losses > 0 {
		p.ProfitFactor = gains / losses
	}
	p.Expectancy = (gains - losses) / float64(len(closed))
	p.SharpeRatio = sharpe(closed)
	p.SortinoRatio = sortino(closed)
	p.MaxDrawdown = maxDD(closed)
	return p
}

func sharpe(records []Record) float64 {
	if len(records) < 2 {
		return 0
	}
	mean, variance := 0.0, 0.0
	for _, r := range records {
		mean += r.PnLPct
	}
	mean /= float64(len(records))
	for _, r := range records {
		d := r.PnLPct - mean
		variance += d * d
	}
	std := math.Sqrt(variance / float64(len(records)-1))
	if std == 0 {
		return 0
	}
	return mean / std * math.Sqrt(float64(len(records)))
}

func sortino(records []Record) float64 {
	if len(records) < 2 {
		return 0
	}
	mean, downVar := 0.0, 0.0
	for _, r := range records {
		mean += r.PnLPct
	}
	mean /= float64(len(records))
	count := 0
	for _, r := range records {
		if r.PnLPct < 0 {
			downVar += r.PnLPct * r.PnLPct
			count++
		}
	}
	if count == 0 {
		return 0
	}
	downStd := math.Sqrt(downVar / float64(count))
	if downStd == 0 {
		return 0
	}
	return mean / downStd
}

func maxDD(records []Record) float64 {
	equity, peak, dd := 100.0, 100.0, 0.0
	for _, r := range records {
		equity *= 1 + r.PnLPct/100
		if equity > peak {
			peak = equity
		}
		if d := (peak - equity) / peak * 100; d > dd {
			dd = d
		}
	}
	return dd
}
