// Пакет smc — Smart Money Concepts.
// ote.go — Optimal Trade Entry: зона входа 61.8%–78.6% Фибоначчи.
package smc

// OTEZone — оптимальная зона входа по Фибоначчи.
type OTEZone struct {
	Direction string
	SwingHigh float64
	SwingLow  float64
	Level618  float64
	Level705  float64
	Level786  float64
	OTETop    float64
	OTEBottom float64
}

// CalcOTE вычисляет OTE-зону.
func CalcOTE(direction string, swingHigh, swingLow float64) *OTEZone {
	z := &OTEZone{Direction: direction, SwingHigh: swingHigh, SwingLow: swingLow}
	d := swingHigh - swingLow
	if direction == "Bullish" {
		z.Level618 = swingHigh - d*0.618
		z.Level705 = swingHigh - d*0.705
		z.Level786 = swingHigh - d*0.786
		z.OTEBottom = z.Level786
		z.OTETop = z.Level618
	} else {
		z.Level618 = swingLow + d*0.618
		z.Level705 = swingLow + d*0.705
		z.Level786 = swingLow + d*0.786
		z.OTEBottom = z.Level618
		z.OTETop = z.Level786
	}
	return z
}

// InZone проверяет, находится ли цена в OTE-зоне.
func (z *OTEZone) InZone(price float64) bool {
	return price >= z.OTEBottom && price <= z.OTETop
}
