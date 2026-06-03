package config

// SignalConfig holds all configurable signal quality parameters.
type SignalConfig struct {
	// Score thresholds
	MinScorePublish      float64 // minimum overall score to publish (default 90)
	MinScoreAPlus        float64 // A+ threshold (default 95)
	MinScoreA            float64 // A  threshold (default 90)
	MinScoreB            float64 // B  threshold (default 85)

	// Risk/Reward
	MinRR        float64 // minimum risk-reward ratio (default 3.0)
	GoodRR       float64 // good signal RR (default 4.0)
	StrongRR     float64 // strong signal RR (default 5.0)
	InstitRR     float64 // institutional signal RR (default 7.0)

	// Probability
	MinProbability float64 // minimum win probability % (default 85)

	// Top N coins by Market Opportunity Score
	// Сколько монет ранжировать по рыночной возможности (1 REST вызов для всех)
	TopCoinsLimit int // default 100

	// Сколько монет из топа подвергать ГЛУБОКОМУ анализу (свечи, OI, funding, стакан).
	// Чем меньше — тем меньше REST запросов. Остальные проходят только по тикеру.
	AnalysisLimit int // default 15

	// Количество параллельных воркеров для REST-запросов.
	// Воркеры разделяют один rate limiter — лимит Bybit не превышается.
	// Оптимум: network_latency / token_interval = 300ms / 750ms ≈ 3.
	WorkerCount int // default 3

	// ATR multiplier for stop-loss padding
	SLATRMultiplier float64 // default 1.5

	// TP placement ratio (% of distance to liquidity target)
	TPPlacementRatio float64 // default 0.95

	// Funding rate thresholds
	FundingExtremePosThresh float64 // funding considered extreme positive (default 0.01)
	FundingExtremeNegThresh float64 // funding considered extreme negative (default -0.01)

	// Volatility compression threshold (ATR ratio below which trading is blocked)
	VolatilityCompressionThresh float64 // default 0.3

	// Minimum volume vs MA20 ratio
	MinVolumeRatio float64 // default 1.0

	// Lookback periods (in candles)
	StructureLookback  int // default 100
	LiquidityLookback  int // default 200
	FVGMaxAge          int // max candles since FVG formation (default 50)
	OrderBlockMaxAge   int // max candles since OB formation (default 100)

	// Session weights (0-1 multiplier on signal score)
	LondonSessionWeight  float64 // default 1.0
	NYSessionWeight      float64 // default 0.95
	OverlapSessionWeight float64 // default 1.0
	AsiaSessionWeight    float64 // default 0.80
	OffHoursWeight       float64 // default 0.70

	// Scoring weights (must sum to 1.0)
	LiquidityWeight  float64 // default 0.30
	StructureWeight  float64 // default 0.25
	FVGOBWeight      float64 // default 0.15
	OIWeight         float64 // default 0.15
	VolumeWeight     float64 // default 0.15

	// Spoofing detection sensitivity (0-1)
	SpoofingSensitivity float64 // default 0.7

	// Footprint imbalance threshold ratio
	FootprintImbalanceThresh float64 // default 3.0

	// Equal highs/lows tolerance %
	EqualLevelsTolerance float64 // default 0.0015

	// ---- Order Block фильтры ----
	OBMinFreshness       float64 // только нетронутые OB (default 1.0)
	OBMinVolumeStrength  float64 // мин. объём OB к среднему (default 1.5)
	OBMaxRetestCount     int     // макс. ретестов (default 0 = первое касание)
	OBMinDisplacement    float64 // мин. displacement за OB в ATR (default 2.0)

	// ---- Режим торговли ----
	// "intraday" — OB на 5M/15M; "swing" — OB на 1H/4H; "both" — оба режима
	TradeMode string // default "both"

	// ---- Число тейков ----
	TPCount int // сколько TP рассчитывать (1–3, default 2)

	// ---- Сопровождение сделки ----
	TradeManagement       bool    // авто-BE и мониторинг позиций (default true)
	BreakevenAtRR         float64 // при каком RR переносить SL в BE (default 1.0)
	NotifyOnTP            bool    // уведомление при TP (default true)
	NotifyOnSL            bool    // уведомление при SL (default true)
	NotifyOnBE            bool    // уведомление при BE (default true)

	// ---- Зоны ожидания (pending) ----
	// Сигнал отправляется только когда цена входит в зону OB
	PendingZoneCheckSec      int     // интервал проверки, сек (default 15)
	PendingZoneMaxAgeSec     int     // макс. возраст зоны, сек (default 7200 = 2ч)
	PendingEntryTolerancePct float64 // допуск входа в зону (default 0.001 = 0.1%)

	// ---- Тайминги обновления данных ----

	// Полное сканирование рынка (ранжирование + анализ всех топ-монет)
	ScanIntervalSec int // интервал полного скана, сек (default 30)

	// Быстрые REST-данные: тикеры, OI, funding, L/S ratio
	// Обновляются чаще скана, чтобы данные были свежими к моменту анализа
	FastDataIntervalSec int // default 15 сек

	// Свечи по таймфреймам (REST)
	// 1M и 5M грузятся чаще — это рабочие таймфреймы для точного входа
	Candles1MCandleIntervalSec  int // default 60 сек (каждую минуту)
	Candles5MCandleIntervalSec  int // default 60 сек
	Candles15MCandleIntervalSec int // default 120 сек (каждые 2 мин)
	CandlesHTFIntervalSec       int // 1H и выше: default 300 сек (каждые 5 мин)

	// Стакан и лента сделок — реальное время через WebSocket (0 = WS)
	OrderBookUpdateMs int // 0 = WebSocket реалтайм
	TradesUpdateMs    int // 0 = WebSocket реалтайм

	// Данные по деривативам (OI история, funding история)
	DerivativesIntervalSec int // default 30 сек

	// Ликвидации (REST-пуллинг)
	LiquidationsIntervalSec int // default 30 сек
}

func DefaultSignalConfig() SignalConfig {
	return SignalConfig{
		MinScorePublish:     90,
		MinScoreAPlus:       95,
		MinScoreA:           90,
		MinScoreB:           85,
		MinRR:               3.0,
		GoodRR:              4.0,
		StrongRR:            5.0,
		InstitRR:            7.0,
		MinProbability:      85,
		TopCoinsLimit:       100,
		AnalysisLimit:       15,
		WorkerCount:         3,
		SLATRMultiplier:     1.5,
		TPPlacementRatio:    0.95,
		FundingExtremePosThresh: 0.01,
		FundingExtremeNegThresh: -0.01,
		VolatilityCompressionThresh: 0.3,
		MinVolumeRatio:      1.0,
		StructureLookback:   100,
		LiquidityLookback:   200,
		FVGMaxAge:           50,
		OrderBlockMaxAge:    100,
		LondonSessionWeight: 1.0,
		NYSessionWeight:     0.95,
		OverlapSessionWeight: 1.0,
		AsiaSessionWeight:   0.80,
		OffHoursWeight:      0.70,
		LiquidityWeight:     0.30,
		StructureWeight:     0.25,
		FVGOBWeight:         0.15,
		OIWeight:            0.15,
		VolumeWeight:        0.15,
		SpoofingSensitivity: 0.7,
		FootprintImbalanceThresh: 3.0,
		EqualLevelsTolerance: 0.0015,
		// Order Block фильтры
		OBMinFreshness:       1.0,
		OBMinVolumeStrength:  1.5,
		OBMaxRetestCount:     0,
		OBMinDisplacement:    2.0,
		// Режим и тейки
		TradeMode: "both",
		TPCount:   2,
		// Сопровождение
		TradeManagement:   true,
		BreakevenAtRR:     1.0,
		NotifyOnTP:        true,
		NotifyOnSL:        true,
		NotifyOnBE:        true,
		// Pending зоны
		PendingZoneCheckSec:      60,
		PendingZoneMaxAgeSec:     7200,
		PendingEntryTolerancePct: 0.001,
		ScanIntervalSec:             30,
		FastDataIntervalSec:         15,
		Candles1MCandleIntervalSec:  60,
		Candles5MCandleIntervalSec:  60,
		Candles15MCandleIntervalSec: 120,
		CandlesHTFIntervalSec:       300,
		OrderBookUpdateMs:           0,
		TradesUpdateMs:              0,
		DerivativesIntervalSec:      30,
		LiquidationsIntervalSec:     30,
	}
}
