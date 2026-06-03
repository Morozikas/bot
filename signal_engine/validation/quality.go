// Пакет validation — валидация сигналов.
// quality.go — финальная проверка качества сигнала перед публикацией.
package validation

import "bybit-elite-signal/config"

// QualityCheck — результат проверки качества.
type QualityCheck struct {
	Passed  bool
	Reasons []string // причины блокировки
}

// Validate проверяет, что сигнал соответствует всем требованиям публикации.
func Validate(
	score, probability, rr float64,
	hasLiquidity, hasSweep, hasBOSorCHOCH bool,
	hasDisplacement, hasFVGorOB bool,
	oiConfirmed, volumeOK, footprintOK, spoofFree bool,
	cfg *config.SignalConfig,
) *QualityCheck {
	q := &QualityCheck{Passed: true}

	check := func(ok bool, reason string) {
		if !ok {
			q.Passed = false
			q.Reasons = append(q.Reasons, reason)
		}
	}

	check(score >= cfg.MinScorePublish, "Score < "+ftoa(cfg.MinScorePublish))
	check(probability >= cfg.MinProbability, "Probability < "+ftoa(cfg.MinProbability))
	check(rr >= cfg.MinRR, "RR < "+ftoa(cfg.MinRR))
	check(hasLiquidity, "Нет ликвидности")
	check(hasSweep, "Нет снятия ликвидности")
	check(hasBOSorCHOCH, "Нет BOS или CHOCH")
	check(hasDisplacement, "Нет Displacement")
	check(hasFVGorOB, "Нет FVG или OB")
	check(oiConfirmed, "OI не подтверждён")
	check(volumeOK, "Объём не подтверждён")
	check(footprintOK, "Footprint не подтверждён")
	check(spoofFree, "Обнаружен спуфинг")

	return q
}

func ftoa(f float64) string {
	if f == float64(int(f)) {
		return itoa(int(f))
	}
	return "?"
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	s := ""
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	return s
}
