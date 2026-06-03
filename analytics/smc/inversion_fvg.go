// Пакет smc — Smart Money Concepts.
// inversion_fvg.go — Inversion FVG: FVG, который был протестирован и изменил полярность.
package smc

import "bybit-elite-signal/api"

// InversionFVG — FVG, сменивший полярность после теста.
type InversionFVG struct {
	OriginalType FVGType
	Top          float64
	Bottom       float64
	NewBias      string // "Bullish" | "Bearish"
	Quality      float64
}

// DetectInversionFVG находит FVG, которые были протестированы и перевернулись.
func DetectInversionFVG(candles []api.Candle, maxAge int) []InversionFVG {
	fvgs := DetectFVG(candles, maxAge)
	result := make([]InversionFVG, 0)
	for _, fvg := range fvgs {
		if fvg.RetestCount >= 1 && !fvg.IsFilled {
			// Тестированный, но незаполненный FVG меняет полярность
			newBias := "Bullish"
			if fvg.Type == BullishFVG {
				newBias = "Bearish"
			}
			result = append(result, InversionFVG{
				OriginalType: fvg.Type,
				Top: fvg.Top, Bottom: fvg.Bottom,
				NewBias: newBias,
				Quality: fvg.Quality * 0.85,
			})
		}
	}
	return result
}
