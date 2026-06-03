// Пакет scoring — финальная оценка сигнала.
// final.go — взвешенная сумма компонентов + штрафы + жёсткие блокировки.
//
// Жёсткие блокировки (return nil):
//   - Компрессия волатильности
//   - Нет ликвидности
//   - Нет свипа ликвидности
//
// Штрафы к Score (не блокируют, снижают оценку):
//   - Объём ниже MA20: −5
//   - OI не подтверждает направление: −5
//
// Убраны из pipeline полностью:
//   - Footprint (оценочные данные, ненадёжны как фильтр)
//   - Spoofing Detection (слишком много ложных срабатываний)
package scoring

import "bybit-elite-signal/config"

// Grade — буквенная оценка сигнала.
type Grade string

const (
	GradeAPlus Grade = "A+"
	GradeA     Grade = "A"
	GradeB     Grade = "B"
	GradeC     Grade = "C"
)

// Input — все компоненты для финального расчёта.
type Input struct {
	LiquidityScore  float64
	StructureScore  float64
	FVGOBScore      float64
	OIScore         float64
	VolumeScore     float64
	FundingAdj      float64
	CorrAdj         float64
	SessionWeight   float64

	// Жёсткие блокировщики
	IsCompressed bool // компрессия волатильности → блокировка
	HasLiquidity bool // есть уровни ликвидности → требуется
	HasSweep     bool // есть свип ликвидности  → требуется

	// Мягкие условия (штраф −5 если не выполнено)
	VolumeOK    bool // объём ≥ MA20
	OIDirection bool // OI подтверждает направление
}

// Result — результат скоринга.
type Result struct {
	Total       float64
	Breakdown   map[string]float64
	Grade       Grade
	Probability float64
	Blocked     bool
	BlockReason string
}

// Calculator вычисляет финальный Score.
type Calculator struct {
	cfg *config.SignalConfig
}

// NewCalculator создаёт Calculator.
func NewCalculator(cfg *config.SignalConfig) *Calculator {
	return &Calculator{cfg: cfg}
}

// Calculate вычисляет итоговый Score.
func (c *Calculator) Calculate(inp Input) *Result {
	r := &Result{Breakdown: make(map[string]float64)}

	// ── Жёсткие блокировки ───────────────────────────────────────────────────
	if inp.IsCompressed {
		return c.block("Компрессия волатильности")
	}
	if !inp.HasLiquidity {
		return c.block("Нет ликвидности")
	}
	if !inp.HasSweep {
		return c.block("Нет снятия ликвидности")
	}

	// ── Взвешенная сумма компонентов ─────────────────────────────────────────
	r.Breakdown["Liquidity"] = inp.LiquidityScore
	r.Breakdown["Structure"] = inp.StructureScore
	r.Breakdown["FVG_OB"] = inp.FVGOBScore
	r.Breakdown["OI"] = inp.OIScore
	r.Breakdown["Volume"] = inp.VolumeScore

	total := inp.LiquidityScore*c.cfg.LiquidityWeight +
		inp.StructureScore*c.cfg.StructureWeight +
		inp.FVGOBScore*c.cfg.FVGOBWeight +
		inp.OIScore*c.cfg.OIWeight +
		inp.VolumeScore*c.cfg.VolumeWeight

	// ── Штрафы (не блокируют, только снижают Score) ──────────────────────────
	if !inp.VolumeOK {
		total -= 5 // объём ниже MA20
	}
	if !inp.OIDirection {
		total -= 5 // OI не подтверждает направление
	}

	// ── Корректировки ────────────────────────────────────────────────────────
	total += inp.FundingAdj + inp.CorrAdj
	total *= inp.SessionWeight
	total = clampSc(total, 0, 100)

	r.Total = total
	r.Probability = clampSc(total*0.6+40, 0, 99)

	switch {
	case total >= c.cfg.MinScoreAPlus:
		r.Grade = GradeAPlus
	case total >= c.cfg.MinScoreA:
		r.Grade = GradeA
	case total >= c.cfg.MinScoreB:
		r.Grade = GradeB
	default:
		r.Grade = GradeC
	}
	return r
}

func (c *Calculator) block(reason string) *Result {
	return &Result{Blocked: true, BlockReason: reason, Breakdown: make(map[string]float64)}
}

func clampSc(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
