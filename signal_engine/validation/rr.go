// Пакет validation — валидация сигналов.
// rr.go — проверка Risk/Reward по полностью настраиваемым порогам из конфига.
//
// Все пороги берутся из config.SignalConfig:
//   MinRR    — минимально допустимый RR (default 3.0)
//   GoodRR   — хороший сигнал (default 4.0)
//   StrongRR — сильный сигнал (default 5.0)
//   InstitRR — институциональный сигнал (default 7.0)
package validation

import (
	"math"

	"bybit-elite-signal/config"
)

// RRResult — результат проверки соотношения риск/прибыль.
type RRResult struct {
	Value   float64
	Allowed bool
	Grade   string // "Institutional" | "Strong" | "Good" | "Minimum" | "Rejected"
}

// CheckRR вычисляет RR и проверяет по настраиваемым порогам из конфига.
func CheckRR(entryPrice, sl, tp float64, cfg *config.SignalConfig) *RRResult {
	risk := math.Abs(entryPrice - sl)
	reward := math.Abs(entryPrice - tp)
	if risk == 0 {
		return &RRResult{Value: 0, Allowed: false, Grade: "Rejected"}
	}

	rr := math.Round(reward/risk*10) / 10
	r := &RRResult{Value: rr, Allowed: rr >= cfg.MinRR}

	switch {
	case rr >= cfg.InstitRR:
		r.Grade = "Institutional" // ≥ 7.0 по умолчанию
	case rr >= cfg.StrongRR:
		r.Grade = "Strong" // ≥ 5.0
	case rr >= cfg.GoodRR:
		r.Grade = "Good" // ≥ 4.0
	case rr >= cfg.MinRR:
		r.Grade = "Minimum" // ≥ 3.0 (минимально допустимый)
	default:
		r.Grade = "Rejected"
	}
	return r
}

// CheckRRValue — упрощённая версия для BestEffort (/scan), принимает явный minRR.
// Используется когда не нужны все пороги (например, для отображения любого сетапа).
func CheckRRValue(entryPrice, sl, tp, minRR float64) *RRResult {
	risk := math.Abs(entryPrice - sl)
	reward := math.Abs(entryPrice - tp)
	if risk == 0 {
		return &RRResult{Value: 0, Allowed: false, Grade: "Rejected"}
	}
	rr := math.Round(reward/risk*10) / 10
	return &RRResult{
		Value:   rr,
		Allowed: rr >= minRR,
		Grade:   rrGrade(rr, minRR),
	}
}

func rrGrade(rr, minRR float64) string {
	switch {
	case rr >= 7.0:
		return "Institutional"
	case rr >= 5.0:
		return "Strong"
	case rr >= 4.0:
		return "Good"
	case rr >= minRR:
		return "Minimum"
	default:
		return "Rejected"
	}
}
