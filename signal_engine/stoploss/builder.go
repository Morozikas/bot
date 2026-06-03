// Пакет stoploss — расчёт стоп-лосса по правилам профессионального дейтрейдинга.
//
// Правила:
//   - SL за структурной точкой инвалидации (OB/FVG нижняя граница + буфер ATR×0.3)
//   - ЗАПРЕЩЕНО ставить SL прямо в очевидной зоне стопов толпы (Equal High/Low)
//     → сдвигаем ЗА пул ликвидности
//   - Для intraday: SL не более 2.5% от входа. Больше — сделка отменяется.
//   - Максимум по ATR: не дальше ATR×2 (компактный стоп)
package stoploss

import (
	"math"

	"bybit-elite-signal/analytics/liquidity"
	"bybit-elite-signal/signal_engine/entry"
)

// MaxStopPct — максимально допустимый стоп для intraday торговли (2.5%).
const MaxStopPct = 0.025

// StopLevel — рассчитанный стоп-лосс.
type StopLevel struct {
	Price   float64
	Pct     float64 // расстояние от входа в %
	Method  string
	Valid   bool   // false = стоп слишком большой, сделку отменить
	Reason  string
}

// Build рассчитывает стоп за структурной точкой инвалидации.
// zone:   зона входа (FVG/OB) — SL ставится ЗА её нижней/верхней границей
// atr14:  ATR(14) рабочего таймфрейма
// lmap:   карта ликвидности для обхода пулов стопов
func Build(direction string, zone *entry.ZoneEntry, entryPrice, atr14 float64, lmap *liquidity.LiquidityMap) *StopLevel {
	if zone == nil {
		return buildFallback(direction, entryPrice, atr14)
	}

	buf := atr14 * 0.3 // небольшой буфер за границей зоны

	var rawSL float64
	switch direction {
	case "Long":
		// SL ниже нижней границы зоны входа
		rawSL = zone.ZoneBot - buf

	case "Short":
		// SL выше верхней границы зоны входа
		rawSL = zone.ZoneTop + buf

	default:
		return buildFallback(direction, entryPrice, atr14)
	}

	// Сдвигаем SL ЗА ближайший пул ликвидности если он попадает в него
	rawSL = moveBehindLiquidityPool(rawSL, direction, lmap, atr14)

	// Проверяем что SL не дальше ATR×2
	rawSL = capByATR(rawSL, direction, entryPrice, atr14)

	// Проверяем intraday лимит 2.5%
	slPct := math.Abs(entryPrice-rawSL) / entryPrice
	if slPct > MaxStopPct {
		return &StopLevel{
			Price:  rawSL,
			Pct:    slPct * 100,
			Method: zone.Source + "Boundary",
			Valid:  false,
			Reason: "стоп > 2.5% — сделка не подходит для дейтрейдинга",
		}
	}

	return &StopLevel{
		Price:  rawSL,
		Pct:    slPct * 100,
		Method: "за " + zone.Source,
		Valid:  true,
	}
}

// moveBehindLiquidityPool сдвигает SL за пул ликвидности.
// Если SL попадает прямо на Equal High/Low — там охота за стопами.
// Нужно разместить ЗА этим уровнем.
func moveBehindLiquidityPool(sl float64, direction string, lmap *liquidity.LiquidityMap, atr float64) float64 {
	if lmap == nil {
		return sl
	}
	tol := atr * 0.4 // допуск попадания в зону

	switch direction {
	case "Long":
		// Для лонга проверяем равные минимумы ниже SL
		for _, lvl := range lmap.EqualLows {
			if math.Abs(sl-lvl.Price) < tol && lvl.Price < sl {
				// SL попал в пул — сдвигаем ниже
				sl = lvl.Price - atr*0.3
			}
		}
	case "Short":
		for _, lvl := range lmap.EqualHighs {
			if math.Abs(sl-lvl.Price) < tol && lvl.Price > sl {
				sl = lvl.Price + atr*0.3
			}
		}
	}
	return sl
}

// capByATR ограничивает дальность стопа: не дальше ATR×2 от входа.
func capByATR(sl float64, direction string, entryPrice, atr float64) float64 {
	maxDist := atr * 2.0
	switch direction {
	case "Long":
		minSL := entryPrice - maxDist
		if sl < minSL {
			return minSL
		}
	case "Short":
		maxSL := entryPrice + maxDist
		if sl > maxSL {
			return maxSL
		}
	}
	return sl
}

func buildFallback(direction string, entryPrice, atr14 float64) *StopLevel {
	offset := atr14 * 1.5
	var price float64
	switch direction {
	case "Long":
		price = entryPrice - offset
	case "Short":
		price = entryPrice + offset
	default:
		price = entryPrice - offset
	}
	pct := offset / entryPrice
	valid := pct <= MaxStopPct
	reason := ""
	if !valid {
		reason = "стоп ATR×1.5 > 2.5% — сделка не подходит для дейтрейдинга"
	}
	return &StopLevel{Price: price, Pct: pct * 100, Method: "ATR×1.5", Valid: valid, Reason: reason}
}
