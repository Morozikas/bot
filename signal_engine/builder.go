// Пакет signalengine — движок генерации сигналов.
// builder.go — сборка полного сигнала из результатов всех аналитических модулей.
package signalengine

import (
	"fmt"
	"math"
	"strings"
	"time"

	"bybit-elite-signal/analytics/correlations"
	"bybit-elite-signal/analytics/derivatives"
	"bybit-elite-signal/analytics/liquidity"
	"bybit-elite-signal/analytics/smc"
	"bybit-elite-signal/analytics/structure"
	"bybit-elite-signal/analytics/volatility"
	"bybit-elite-signal/analytics/volume"
	"bybit-elite-signal/api"
	"bybit-elite-signal/config"
	"bybit-elite-signal/scoring"
	"bybit-elite-signal/signal_engine/entry"
	"bybit-elite-signal/signal_engine/probability"
	"bybit-elite-signal/signal_engine/stoploss"
	"bybit-elite-signal/signal_engine/takeprofit"
	"bybit-elite-signal/signal_engine/validation"

	"github.com/google/uuid"
)

// Signal — готовый торговый сигнал.
type Signal struct {
	ID          string
	Symbol      string
	Direction   string // "Long" | "Short"
	Entry       float64
	StopLoss    float64
	TakeProfit  float64
	MarkPrice   float64 // mark price на момент генерации
	ATR14_1H    float64 // ATR(14) на 1H — для расчёта дедлайна
	RR          float64
	Score       float64
	Grade       scoring.Grade
	Probability float64
	Setup       string
	CreatedAt   time.Time

	// Флаги подтверждений
	LiquiditySweep  bool
	HasBOS          bool
	HasCHOCH        bool
	HasDisplacement bool
	HasFVG          bool
	HasOB           bool
	OIConfirmed     bool
	VolumeConfirmed bool
	FootprintConf   bool
	SpoofFree       bool
}

// BuildInput — входные данные для построения сигнала.
type BuildInput struct {
	Symbol    string
	Direction string

	// Рыночные данные
	Candles1H   []api.Candle
	Candles15M  []api.Candle
	Candles5M   []api.Candle
	Candles1M   []api.Candle
	BTCCandles  []api.Candle
	ETHCandles  []api.Candle
	Trades      []api.Trade
	OrderBook   *api.OrderBook
	OIHistory   []api.OpenInterest
	FundingHist []api.FundingRate
	LSHistory   []api.LongShortRatio
	Liquidations []api.LiquidationRecord
	MarkPrice   float64
}

// Builder строит Signal из аналитических данных.
type Builder struct {
	cfg     *config.AppConfig
	scorer  *scoring.Calculator
}

// NewBuilder создаёт Builder.
func NewBuilder(cfg *config.AppConfig) *Builder {
	return &Builder{
		cfg:    cfg,
		scorer: scoring.NewCalculator(&cfg.Signal),
	}
}

// Build запускает полный конвейер и применяет ВСЕ пороговые фильтры.
// Используется автосканером — публикует только сигналы прошедшие все критерии.
func (b *Builder) Build(inp BuildInput) *Signal {
	return b.buildInternal(inp, true)
}

// BuildBestEffort запускает ИДЕНТИЧНЫЙ конвейер, но пропускает пороговые фильтры.
// Используется /scan — возвращает сигнал с наибольшим Score независимо от порогов.
// Расчёт Entry/SL/TP/Score полностью идентичен Build().
func (b *Builder) BuildBestEffort(inp BuildInput) *Signal {
	return b.buildInternal(inp, false)
}

// buildInternal — единый аналитический конвейер для обоих публичных методов.
// applyFilters=true:  публикуемый сигнал (MinScore, MinRR, MinProbability, qualCheck)
// applyFilters=false: лучший сетап для /scan (те же расчёты, нет порогового отсева)
func (b *Builder) buildInternal(inp BuildInput, applyFilters bool) *Signal {
	cfg := &b.cfg.Signal

	if len(inp.Candles15M) < 20 || len(inp.Candles1H) < 20 {
		return nil
	}

	// ── Шаг 1: Волатильность ─────────────────────────────────────────────────
	vol := volatility.Analyze(inp.Candles15M)
	if vol.IsCompressed {
		return nil // компрессия — нет торговли ни в каком режиме
	}

	// ── Шаг 2: Структура рынка (1H + 15M) ───────────────────────────────────
	str1h := structure.AnalyzeTrend(inp.Symbol, "1H", inp.Candles1H, cfg.StructureLookback)
	str15m := structure.AnalyzeTrend(inp.Symbol, "15M", inp.Candles15M, cfg.StructureLookback)

	// ── Шаг 3: Направление ───────────────────────────────────────────────────
	direction := inp.Direction
	if direction == "" {
		direction = detectDirection(str1h, str15m)
		if direction == "" {
			if applyFilters {
				return nil // автосканер: направление обязательно
			}
			// /scan: определяем по EMA если структура не дала ответа
			direction = directionFromCandles(inp.Candles1H)
			if direction == "" {
				return nil
			}
		}
	}

	// ── Шаг 4: Ликвидность ───────────────────────────────────────────────────
	lmap := liquidity.Build(inp.Candles1H, inp.Liquidations, cfg)
	// Для автосканера (applyFilters=true): свип обязателен
	// Для /scan (applyFilters=false): показываем сетап даже без свипа — Score снизится
	if applyFilters && (!lmap.HasLiquidity || lmap.Sweep == nil || !lmap.Sweep.Detected) {
		return nil
	}
	// Без свипа — продолжаем, но HasSweep=false снизит Score в скорере

	// ── Шаг 5: SMC зоны ──────────────────────────────────────────────────────
	fvgs := smcFVGs(inp.Candles15M, inp.Candles5M, direction, cfg.FVGMaxAge)
	obs := smcOBs(inp.Candles1H, inp.Candles15M, direction, cfg.OrderBlockMaxAge)

	// Displacement не используется как жёсткий фильтр — фиксируем факт для Setup-строки
	disp := detectDisplacement(inp.Candles5M)

	// ── Шаг 7: Деривативы ────────────────────────────────────────────────────
	oiResult := derivatives.AnalyzeOI(inp.OIHistory, inp.Candles1H)
	fundResult := derivatives.AnalyzeFunding(inp.FundingHist, cfg.FundingExtremePosThresh, cfg.FundingExtremeNegThresh)
	_ = derivatives.AnalyzeLSRatio(inp.LSHistory)

	// ── Шаг 8: Объём ─────────────────────────────────────────────────────────
	volStats := volume.Analyze(inp.Candles15M, cfg.MinVolumeRatio)
	volProfile := volume.Build(inp.Candles1H, 100)
	// Объём ниже MA20 → штраф −5 к Score, не блокировка

	// ── Корреляции ───────────────────────────────────────────────────────────
	corrResult := correlations.AnalyzeBTCETH(inp.BTCCandles, inp.ETHCandles, direction)

	// ── Скоринг ──────────────────────────────────────────────────────────────
	oiOK := (direction == "Long" && oiResult.AllowsLong) || (direction == "Short" && oiResult.AllowsShort)
	scoreResult := b.scorer.Calculate(scoring.Input{
		LiquidityScore: liquidity.Score(lmap),
		StructureScore: structure.StructureScore(str1h),
		FVGOBScore:     fvgOBScore(fvgs, obs),
		OIScore:        oiResult.Score,
		VolumeScore:    volume.Score(volStats),
		FundingAdj:     fundResult.ScoreAdj,
		CorrAdj:        corrResult.ScoreAdj,
		SessionWeight:  scoring.SessionWeight(cfg),
		IsCompressed:   vol.IsCompressed,
		HasLiquidity:   lmap.HasLiquidity,
		HasSweep:       lmap.Sweep != nil && lmap.Sweep.Detected,
		VolumeOK:       volStats.IsAboveMA20,  // штраф −5 если ниже MA20
		OIDirection:    oiOK,                  // штраф −5 если OI против
	})

	// Пороговый фильтр по Score — только для автосканера
	if applyFilters && (scoreResult.Blocked || scoreResult.Total < cfg.MinScorePublish) {
		return nil
	}

	// ── Шаг 13: Entry / SL / TP ──────────────────────────────────────────────
	entryPt := entry.Select(fvgs, obs, volStats, direction, inp.MarkPrice, vol.ATR14,
		lastSwingHigh(str1h), lastSwingLow(str1h))
	if entryPt == nil || entryPt.Price == 0 {
		return nil
	}

	slLevel := stoploss.Build(direction, entryPt, entryPt.Price, vol.ATR14, lmap)
	if slLevel == nil {
		return nil
	}
	// Для автосканера отменяем если стоп > 2.5% (intraday лимит)
	if applyFilters && !slLevel.Valid {
		return nil
	}

	allOppFVGs := append(
		smcOppositeFVGs(inp.Candles5M, direction, cfg.FVGMaxAge),
		smcOppositeFVGs(inp.Candles15M, direction, cfg.FVGMaxAge)...,
	)
	var tpLevel *takeprofit.TPLevel
	if direction == "Long" {
		tpLevel = takeprofit.SelectLong(entryPt.Price, vol.ATR14, lmap, volProfile, allOppFVGs, cfg.TPPlacementRatio)
	} else {
		tpLevel = takeprofit.SelectShort(entryPt.Price, vol.ATR14, lmap, volProfile, allOppFVGs, cfg.TPPlacementRatio)
	}
	if tpLevel == nil {
		return nil
	}

	// ── Шаг 14: RR ───────────────────────────────────────────────────────────
	rrCheck := validation.CheckRR(entryPt.Price, slLevel.Price, tpLevel.Price, cfg)
	// Для автосканера: RR должен быть ≥ MinRR
	// Для /scan: показываем любой сетап, даже с RR < MinRR
	if applyFilters && !rrCheck.Allowed {
		return nil
	}

	// ── Шаг 15: Вероятность ──────────────────────────────────────────────────
	prob := probability.Estimate(scoreResult.Total, rrCheck.Value)
	if applyFilters && prob < cfg.MinProbability {
		return nil
	}

	// ── Шаг 16: Финальная проверка — только для автосканера ──────────────────
	if applyFilters {
		qualCheck := validation.Validate(
			scoreResult.Total, prob, rrCheck.Value,
			lmap.HasLiquidity, lmap.Sweep.Detected,
			str15m.CHOCH.Detected || str15m.BOS.Detected,
			disp, len(fvgs) > 0 || len(obs) > 0,
			oiOK, volStats.IsAboveMA20,
			true, // footprint убран как фильтр
			true, // spoofing убран как фильтр
			cfg,
		)
		if !qualCheck.Passed {
			return nil
		}
	}

	// ATR1H нужен для расчёта дедлайна сигнала
	atr1h := 0.0
	if len(inp.Candles1H) >= 15 {
		atr1h = volatility.ATR14(inp.Candles1H)
	}

	sig := &Signal{
		ID: uuid.New().String(), Symbol: inp.Symbol, Direction: direction,
		Entry: entryPt.Price, StopLoss: slLevel.Price, TakeProfit: tpLevel.Price,
		MarkPrice: inp.MarkPrice, ATR14_1H: atr1h,
		RR: rrCheck.Value, Score: scoreResult.Total, Grade: scoreResult.Grade,
		Probability: prob, CreatedAt: time.Now(),
		LiquiditySweep:  lmap.Sweep != nil && lmap.Sweep.Detected,
		HasBOS:          str15m.BOS.Detected, HasCHOCH: str15m.CHOCH.Detected,
		HasDisplacement: disp, HasFVG: len(fvgs) > 0, HasOB: len(obs) > 0,
		OIConfirmed:     oiOK, VolumeConfirmed: volStats.IsAboveMA20,
		FootprintConf:   true, // убран как фильтр
		SpoofFree:       true, // убран как фильтр
	}
	sig.Setup = buildSetup(sig)
	return sig
}

// ATR1H возвращает ATR(14) на 1H таймфрейме (для расчёта дедлайна).
func (s *Signal) ATR1H() float64 { return s.ATR14_1H }

// directionFromCandles определяет направление по EMA9/EMA21 на 1H.
func directionFromCandles(candles []api.Candle) string {
	if len(candles) < 21 {
		return ""
	}
	k9 := 2.0 / float64(10)
	k21 := 2.0 / float64(22)
	e9, _ := candles[0].Close.Float64()
	e21 := e9
	for i := 1; i < len(candles); i++ {
		c, _ := candles[i].Close.Float64()
		e9 = c*k9 + e9*(1-k9)
		e21 = c*k21 + e21*(1-k21)
	}
	if e9 > e21 {
		return "Long"
	}
	return "Short"
}

// ---- helpers ----

func detectDirection(h1, m15 *structure.TrendResult) string {
	bullH1 := h1.CHOCH.Detected && h1.CHOCH.Direction == "Bullish" || h1.BOS.Detected && h1.BOS.Direction == "Bullish"
	bearH1 := h1.CHOCH.Detected && h1.CHOCH.Direction == "Bearish" || h1.BOS.Detected && h1.BOS.Direction == "Bearish"
	bullM15 := m15.CHOCH.Detected && m15.CHOCH.Direction == "Bullish" || m15.BOS.Detected && m15.BOS.Direction == "Bullish"
	bearM15 := m15.CHOCH.Detected && m15.CHOCH.Direction == "Bearish" || m15.BOS.Detected && m15.BOS.Direction == "Bearish"
	if bullH1 && bullM15 {
		return "Long"
	}
	if bearH1 && bearM15 {
		return "Short"
	}
	return ""
}

// lastSwingHigh возвращает цену последнего свингового максимума из анализа структуры.
func lastSwingHigh(r *structure.TrendResult) float64 {
	if r == nil || len(r.Highs) == 0 {
		return 0
	}
	return r.Highs[len(r.Highs)-1].Price
}

// lastSwingLow возвращает цену последнего свингового минимума.
func lastSwingLow(r *structure.TrendResult) float64 {
	if r == nil || len(r.Lows) == 0 {
		return 0
	}
	return r.Lows[len(r.Lows)-1].Price
}

func smcFVGs(candles15m, candles5m []api.Candle, direction string, maxAge int) []smc.FVG {
	all := append(smc.DetectFVG(candles15m, maxAge), smc.DetectFVG(candles5m, maxAge)...)
	result := make([]smc.FVG, 0)
	for _, f := range all {
		if (direction == "Long" && f.Type == smc.BullishFVG) ||
			(direction == "Short" && f.Type == smc.BearishFVG) {
			result = append(result, f)
		}
	}
	return result
}

func smcOBs(candles1h, candles15m []api.Candle, direction string, maxAge int) []smc.OrderBlock {
	all := append(smc.DetectOrderBlocks(candles1h, maxAge), smc.DetectOrderBlocks(candles15m, maxAge)...)
	result := make([]smc.OrderBlock, 0)
	for _, o := range all {
		if (direction == "Long" && o.Type == smc.BullishOB) ||
			(direction == "Short" && o.Type == smc.BearishOB) {
			result = append(result, o)
		}
	}
	return result
}

// smcOppositeFVGs возвращает FVG в направлении ПРОТИВОПОЛОЖНОМ сигналу.
// Используются как цели для тейк-профита:
//   Long сигнал  → возвращаем медвежьи FVG выше входа (там цена может развернуться)
//   Short сигнал → возвращаем бычьи FVG ниже входа
func smcOppositeFVGs(candles []api.Candle, direction string, maxAge int) []smc.FVG {
	all := smc.DetectFVG(candles, maxAge)
	result := make([]smc.FVG, 0)
	for _, f := range all {
		if direction == "Long" && f.Type == smc.BearishFVG {
			result = append(result, f)
		}
		if direction == "Short" && f.Type == smc.BullishFVG {
			result = append(result, f)
		}
	}
	return result
}

func detectDisplacement(candles5m []api.Candle) bool {
	if len(candles5m) < 3 {
		return false
	}
	avgBody := 0.0
	for _, c := range candles5m[len(candles5m)-10:] {
		o, _ := c.Open.Float64()
		cl, _ := c.Close.Float64()
		b := math.Abs(cl - o)
		avgBody += b
	}
	avgBody /= 10
	last := candles5m[len(candles5m)-1]
	o, _ := last.Open.Float64()
	cl, _ := last.Close.Float64()
	return avgBody > 0 && math.Abs(cl-o) >= 2*avgBody
}

func fvgOBScore(fvgs []smc.FVG, obs []smc.OrderBlock) float64 {
	best := 0.0
	for _, f := range fvgs {
		if f.Quality > best {
			best = f.Quality
		}
	}
	for _, o := range obs {
		if o.Quality > best {
			best = o.Quality
		}
	}
	return best
}

func buildSetup(s *Signal) string {
	parts := make([]string, 0)
	if s.LiquiditySweep {
		parts = append(parts, "Liquidity Sweep")
	}
	if s.HasFVG {
		if s.Direction == "Long" {
			parts = append(parts, "Bullish FVG")
		} else {
			parts = append(parts, "Bearish FVG")
		}
	}
	if s.HasOB {
		if s.Direction == "Long" {
			parts = append(parts, "Bullish OB")
		} else {
			parts = append(parts, "Bearish OB")
		}
	}
	if s.HasCHOCH {
		parts = append(parts, "CHOCH")
	} else if s.HasBOS {
		parts = append(parts, "BOS")
	}
	if s.OIConfirmed {
		parts = append(parts, "OI Expansion")
	}
	if s.FootprintConf {
		parts = append(parts, "Absorption")
	}
	return strings.Join(parts, " + ")
}

func init() { _ = fmt.Sprintf }
