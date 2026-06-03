// Пакет derivatives — аналитика деривативов.
// squeeze.go — обнаружение Short/Long Squeeze: резкое движение против толпы.
package derivatives

// SqueezeResult — результат анализа потенциального сквиза.
type SqueezeResult struct {
	ShortSqueezePotential bool
	LongSqueezePotential  bool
	Intensity             float64 // 0-100
}

// DetectSqueeze оценивает потенциал сквиза.
// Шорт-сквиз: много шортов + рост цены + рост OI.
// Лонг-сквиз: много лонгов + падение цены + рост OI.
func DetectSqueeze(ls *LSRatioResult, oi *OIResult) *SqueezeResult {
	r := &SqueezeResult{}
	if ls == nil || oi == nil {
		return r
	}

	// Шорт-сквиз: ratio < 0.6 (много шортов) и Short Build-Up
	if ls.LSRatio < 0.6 && oi.Condition == ShortBuildUp {
		r.ShortSqueezePotential = true
		r.Intensity = clampD((0.6-ls.LSRatio)*100, 0, 100)
	}

	// Лонг-сквиз: ratio > 2.0 (много лонгов) и Long Unwinding
	if ls.LSRatio > 2.0 && oi.Condition == LongUnwinding {
		r.LongSqueezePotential = true
		r.Intensity = clampD((ls.LSRatio-2.0)*50, 0, 100)
	}
	return r
}
