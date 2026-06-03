// Пакет entry — расчёт зоны входа по правилам профессионального дейтрейдинга.
//
// Приоритет зон входа:
//   1. Order Block в направлении сделки
//   2. Незакрытый FVG (имбаланс), куда цена вернётся
//   3. OTE — зона Фибоначчи 61.8%–78.6% последнего свинга
//   4. Уровень поддержки/сопротивления
//
// Жёсткие правила:
//   Long:  entry < currentPrice (никогда выше рынка)
//   Short: entry > currentPrice (никогда ниже рынка)
//   Максимальное удаление от цены: 2% (intraday ордер должен исполниться сегодня)
package entry

import (
	"math"

	"bybit-elite-signal/analytics/smc"
	"bybit-elite-signal/analytics/volume"
)

// ZoneEntry — зона входа с границами и условием подтверждения.
type ZoneEntry struct {
	Price     float64 // идеальная цена внутри зоны (для лимитного ордера)
	ZoneTop   float64 // верхняя граница зоны
	ZoneBot   float64 // нижняя граница зоны
	Source    string  // "OB" | "FVG" | "OTE" | "Support" | "Market"
	Confirm   string  // условие подтверждения входа
	Quality   float64
	Adjusted  bool
}

// MaxIntraday — максимально допустимое удаление entry от цены для intraday торговли (2%).
const MaxIntraday = 0.02

// Select выбирает лучшую зону входа по приоритету.
// Возвращает nil если нет подходящей зоны в допустимом расстоянии.
func Select(
	fvgs []smc.FVG,
	obs []smc.OrderBlock,
	volStats *volume.VolumeStats,
	direction string,
	markPrice float64,
	atr14 float64,
	swingHigh float64, // последний свинговый максимум (для OTE)
	swingLow float64,  // последний свинговый минимум
) *ZoneEntry {
	maxDist := math.Max(markPrice*MaxIntraday, atr14*2.5)

	// Приоритет 1: Order Block
	if ep := bestOB(obs, direction, markPrice, maxDist); ep != nil {
		return ep
	}

	// Приоритет 2: FVG (незакрытый имбаланс)
	if ep := bestFVG(fvgs, direction, markPrice, maxDist); ep != nil {
		return ep
	}

	// Приоритет 3: OTE зона (Фибоначчи 61.8%–78.6%)
	if ep := oteZone(direction, markPrice, swingHigh, swingLow, maxDist); ep != nil {
		return ep
	}

	// Приоритет 4: VWAP как динамическая поддержка/сопротивление
	if volStats != nil && volStats.VWAP > 0 {
		if ep := vwapSupport(volStats.VWAP, direction, markPrice, maxDist); ep != nil {
			return ep
		}
	}

	// Нет подходящей зоны в intraday диапазоне
	return nil
}

// bestOB возвращает ближайший Order Block нужного направления.
func bestOB(obs []smc.OrderBlock, direction string, markPrice, maxDist float64) *ZoneEntry {
	var best *smc.OrderBlock
	bestDist := math.MaxFloat64

	for i := range obs {
		o := &obs[i]
		if o.Freshness < 0.4 {
			continue // сильно митигированный OB пропускаем
		}
		var entryEdge float64
		switch direction {
		case "Long":
			if o.Top >= markPrice {
				continue // OB выше рынка — не подходит для лонга
			}
			entryEdge = o.Top // входим на верхней границе OB
		case "Short":
			if o.Bottom <= markPrice {
				continue
			}
			entryEdge = o.Bottom
		}
		dist := math.Abs(entryEdge - markPrice)
		if dist > maxDist {
			continue
		}
		if best == nil || dist < bestDist || o.Quality > best.Quality {
			best = o
			bestDist = dist
		}
	}
	if best == nil {
		return nil
	}

	var price float64
	switch direction {
	case "Long":
		// Идеальная точка — 50% от OB (середина), верхняя граница = top, нижняя = bottom
		price = best.Top
	case "Short":
		price = best.Bottom
	}

	return &ZoneEntry{
		Price:   price,
		ZoneTop: best.Top,
		ZoneBot: best.Bottom,
		Source:  "OB",
		Confirm: "реакция + объёмная свеча на границе OB",
		Quality: best.Quality,
	}
}

// bestFVG возвращает ближайший незакрытый FVG нужного направления.
func bestFVG(fvgs []smc.FVG, direction string, markPrice, maxDist float64) *ZoneEntry {
	var best *smc.FVG
	bestDist := math.MaxFloat64

	for i := range fvgs {
		f := &fvgs[i]
		var entryEdge float64
		switch direction {
		case "Long":
			if f.Type != smc.BullishFVG || f.Top >= markPrice {
				continue // FVG выше цены — уже пройден или неподходящий
			}
			entryEdge = f.Top // входим на верхней границе gap (первое касание)
		case "Short":
			if f.Type != smc.BearishFVG || f.Bottom <= markPrice {
				continue
			}
			entryEdge = f.Bottom
		}
		dist := math.Abs(entryEdge - markPrice)
		if dist > maxDist {
			continue
		}
		if best == nil || dist < bestDist {
			best = f
			bestDist = dist
		}
	}
	if best == nil {
		return nil
	}

	var price float64
	switch direction {
	case "Long":
		price = best.Top
	case "Short":
		price = best.Bottom
	}

	return &ZoneEntry{
		Price:   price,
		ZoneTop: best.Top,
		ZoneBot: best.Bottom,
		Source:  "FVG",
		Confirm: "возврат в gap + бычья/медвежья свеча с закрытием внутри",
		Quality: best.Quality,
	}
}

// oteZone рассчитывает зону OTE (Optimal Trade Entry, Fibonacci 61.8%–78.6%).
func oteZone(direction string, markPrice, swingHigh, swingLow, maxDist float64) *ZoneEntry {
	if swingHigh <= 0 || swingLow <= 0 || swingHigh <= swingLow {
		return nil
	}
	diff := swingHigh - swingLow

	switch direction {
	case "Long":
		// После бычьего движения вверх: OTE = коррекция 61.8%–78.6% от swingLow
		// Зона = [swingHigh - diff*0.786, swingHigh - diff*0.618]
		zoneBot := swingHigh - diff*0.786
		zoneTop := swingHigh - diff*0.618
		if zoneTop >= markPrice || markPrice-zoneTop > maxDist {
			return nil
		}
		ideal := swingHigh - diff*0.705 // 70.5% — середина OTE
		return &ZoneEntry{
			Price: ideal, ZoneTop: zoneTop, ZoneBot: zoneBot,
			Source:  "OTE",
			Confirm: "вход в OTE зону + подтверждение реакцией на 1M/5M",
			Quality: 60,
		}

	case "Short":
		// После медвежьего движения: OTE = коррекция 61.8%–78.6% от swingHigh
		zoneTop := swingLow + diff*0.786
		zoneBot := swingLow + diff*0.618
		if zoneBot <= markPrice || zoneBot-markPrice > maxDist {
			return nil
		}
		ideal := swingLow + diff*0.705
		return &ZoneEntry{
			Price: ideal, ZoneTop: zoneTop, ZoneBot: zoneBot,
			Source:  "OTE",
			Confirm: "вход в OTE зону + подтверждение разворотной свечой на 5M",
			Quality: 60,
		}
	}
	return nil
}

// vwapSupport использует VWAP как динамическую поддержку/сопротивление.
func vwapSupport(vwap float64, direction string, markPrice, maxDist float64) *ZoneEntry {
	switch direction {
	case "Long":
		if vwap >= markPrice || markPrice-vwap > maxDist {
			return nil
		}
		buf := markPrice * 0.001
		return &ZoneEntry{
			Price: vwap, ZoneTop: vwap + buf, ZoneBot: vwap - buf,
			Source:  "VWAP",
			Confirm: "ретест VWAP снизу-вверх + объёмная реакция",
			Quality: 40,
		}
	case "Short":
		if vwap <= markPrice || vwap-markPrice > maxDist {
			return nil
		}
		buf := markPrice * 0.001
		return &ZoneEntry{
			Price: vwap, ZoneTop: vwap + buf, ZoneBot: vwap - buf,
			Source:  "VWAP",
			Confirm: "ретест VWAP сверху-вниз + объёмная реакция",
			Quality: 40,
		}
	}
	return nil
}

// IsValidSide проверяет что entry на правильной стороне от текущей цены.
func IsValidSide(entryPrice float64, direction string, markPrice float64) bool {
	switch direction {
	case "Long":
		return entryPrice < markPrice
	case "Short":
		return entryPrice > markPrice
	}
	return false
}
