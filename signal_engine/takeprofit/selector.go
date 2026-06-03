// Пакет takeprofit — расчёт тейк-профита по правилам профессионального дейтрейдинга.
//
// Принципы:
//   - TP ставится ПЕРЕД ближайшим препятствием (не за ним)
//   - Между входом и TP проверяются встречные уровни
//   - Для intraday: цель реалистична в рамках 1–2 сессий
//   - TP не дальше ATR×3 (иначе нереалистично для дня)
//   - Размещать на 95–97% от цели (не заходить в зону полностью)
package takeprofit

import (
	"math"

	"bybit-elite-signal/analytics/liquidity"
	"bybit-elite-signal/analytics/smc"
	"bybit-elite-signal/analytics/volume"
)

// MaxTPMultiplier — максимально допустимое расстояние TP от входа (ATR×3).
const MaxTPMultiplier = 3.0

// TPLevel — рассчитанный тейк-профит.
type TPLevel struct {
	Price    float64
	Source   string
	Distance float64
	Pct      float64
}

// SelectLong выбирает ближайшую достижимую цель для лонга.
// Цель — ближайшее препятствие ВЫШЕ входа, TP ставится ПЕРЕД ним.
func SelectLong(
	entry float64,
	atr14 float64,
	lmap *liquidity.LiquidityMap,
	profile *volume.Profile,
	oppFVGs []smc.FVG, // медвежьи FVG выше (препятствие)
	ratio float64,
) *TPLevel {
	maxTP := entry + atr14*MaxTPMultiplier
	minTP := entry + atr14*0.8

	targets := gatherLongTargets(entry, maxTP, lmap, profile, oppFVGs)
	nearest := nearestAbove(targets, entry)

	if nearest == 0 || nearest < minTP {
		// Нет явного уровня — ATR×1.5 как минимальная внутридневная цель
		nearest = entry + atr14*1.5
		if nearest > maxTP {
			nearest = maxTP
		}
	}

	// TP на 95% пути до цели — не заходим в зону полностью
	tp := entry + (nearest-entry)*ratio
	pct := (tp - entry) / entry * 100

	return &TPLevel{
		Price:    tp,
		Source:   labelLong(nearest, lmap, oppFVGs),
		Distance: tp - entry,
		Pct:      pct,
	}
}

// SelectShort выбирает ближайшую достижимую цель для шорта.
func SelectShort(
	entry float64,
	atr14 float64,
	lmap *liquidity.LiquidityMap,
	profile *volume.Profile,
	oppFVGs []smc.FVG,
	ratio float64,
) *TPLevel {
	minTP := entry - atr14*MaxTPMultiplier
	maxTP := entry - atr14*0.8

	targets := gatherShortTargets(entry, minTP, lmap, profile, oppFVGs)
	nearest := nearestBelow(targets, entry)

	if nearest == 0 || nearest > maxTP {
		nearest = entry - atr14*1.5
		if nearest < minTP {
			nearest = minTP
		}
	}

	tp := entry - (entry-nearest)*ratio
	pct := (entry - tp) / entry * 100

	return &TPLevel{
		Price:    tp,
		Source:   labelShort(nearest, lmap, oppFVGs),
		Distance: entry - tp,
		Pct:      pct,
	}
}

// gatherLongTargets собирает все потенциальные цели выше входа для лонга.
func gatherLongTargets(entry, maxTP float64, lmap *liquidity.LiquidityMap, profile *volume.Profile, oppFVGs []smc.FVG) []float64 {
	targets := make([]float64, 0)

	// Buy Side Liquidity (равные максимумы) — останавливаемся ПЕРЕД пулом
	if lmap != nil {
		for _, lvl := range lmap.EqualHighs {
			if lvl.Price > entry && lvl.Price <= maxTP {
				targets = append(targets, lvl.Price*0.998) // 0.2% перед пулом
			}
		}
		// Медвежьи OB выше — препятствие, ставим TP перед ними
		for _, pool := range lmap.BuySide {
			if pool.Price > entry && pool.Price <= maxTP {
				targets = append(targets, pool.Price*0.997)
			}
		}
	}

	// Медвежьи FVG выше (незакрытые gaps = возможный разворот)
	for _, f := range oppFVGs {
		if f.Type == smc.BearishFVG && f.Bottom > entry && f.Bottom <= maxTP {
			targets = append(targets, f.Bottom*0.998)
		}
	}

	// HVN из Volume Profile (зоны поглощения объёма)
	if profile != nil {
		if hvn := volume.NearestHVNAbove(profile, entry); hvn > 0 && hvn <= maxTP {
			targets = append(targets, hvn*0.998)
		}
	}

	return targets
}

// gatherShortTargets собирает все потенциальные цели ниже входа для шорта.
func gatherShortTargets(entry, minTP float64, lmap *liquidity.LiquidityMap, profile *volume.Profile, oppFVGs []smc.FVG) []float64 {
	targets := make([]float64, 0)

	if lmap != nil {
		for _, lvl := range lmap.EqualLows {
			if lvl.Price < entry && lvl.Price >= minTP {
				targets = append(targets, lvl.Price*1.002) // 0.2% перед пулом
			}
		}
		for _, pool := range lmap.SellSide {
			if pool.Price < entry && pool.Price >= minTP {
				targets = append(targets, pool.Price*1.003)
			}
		}
	}

	for _, f := range oppFVGs {
		if f.Type == smc.BullishFVG && f.Top < entry && f.Top >= minTP {
			targets = append(targets, f.Top*1.002)
		}
	}

	if profile != nil {
		if hvn := volume.NearestHVNBelow(profile, entry); hvn > 0 && hvn >= minTP {
			targets = append(targets, hvn*1.002)
		}
	}

	return targets
}

func nearestAbove(values []float64, base float64) float64 {
	best := 0.0
	for _, v := range values {
		if v > base && (best == 0 || v < best) {
			best = v
		}
	}
	return best
}

func nearestBelow(values []float64, base float64) float64 {
	best := 0.0
	for _, v := range values {
		if v < base && (best == 0 || v > best) {
			best = v
		}
	}
	return best
}

func labelLong(target float64, lmap *liquidity.LiquidityMap, fvgs []smc.FVG) string {
	if lmap != nil {
		for _, lvl := range lmap.EqualHighs {
			if math.Abs(target-lvl.Price*0.998) < lvl.Price*0.005 {
				return "перед BSL (Equal Highs)"
			}
		}
	}
	for _, f := range fvgs {
		if f.Type == smc.BearishFVG && math.Abs(target-f.Bottom*0.998) < f.Bottom*0.005 {
			return "перед Bearish FVG"
		}
	}
	return "HVN / ATR цель"
}

func labelShort(target float64, lmap *liquidity.LiquidityMap, fvgs []smc.FVG) string {
	if lmap != nil {
		for _, lvl := range lmap.EqualLows {
			if math.Abs(target-lvl.Price*1.002) < lvl.Price*0.005 {
				return "перед SSL (Equal Lows)"
			}
		}
	}
	for _, f := range fvgs {
		if f.Type == smc.BullishFVG && math.Abs(target-f.Top*1.002) < f.Top*0.005 {
			return "перед Bullish FVG"
		}
	}
	return "HVN / ATR цель"
}
