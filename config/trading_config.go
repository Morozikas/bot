package config

// TradingConfig controls auto-execution behaviour.
type TradingConfig struct {
	Enabled bool // master switch for auto-trading

	// Default leverage used when user has no per-symbol override
	DefaultLeverage int // default 10

	// Maximum allowed leverage (safety cap)
	MaxLeverage int // default 20

	// Risk per trade as % of account equity
	DefaultRiskPct float64 // default 1.0 (1%)

	// Maximum simultaneous open positions per user
	MaxPositionsPerUser int // default 5

	// Maximum total open positions across all users
	MaxTotalPositions int // default 50

	// Minimum account balance (USDT) required to place a trade
	MinAccountBalance float64 // default 100

	// Order type for entry (always limit in this system)
	EntryOrderType string // "Limit"

	// Reduce-only order for TP/SL
	ReduceOnly bool // default true for TP

	// Time-in-force for limit orders
	TimeInForce string // "GoodTillCancel"

	// Cancel entry order if not filled within N seconds
	EntryOrderTTLSec int // default 300 (5 min)

	// Trailing stop activation distance % from entry
	TrailingStopPct float64 // 0 = disabled

	// Move SL to breakeven when profit reaches X * risk
	BreakevenAt float64 // 0 = disabled, example 1.0 (1:1)

	// Partial close at first milestone (0 = disabled)
	PartialCloseRR     float64 // e.g. 2.0 – close 50% at 2R
	PartialClosePct    float64 // e.g. 0.5 – close 50% of position

	// Maximum drawdown before pausing trading (% of initial balance)
	MaxDailyDrawdownPct float64 // default 5.0

	// Public fallback used for analytics when no API key is configured
	PublicAnalyticsOnly bool
}

func DefaultTradingConfig() TradingConfig {
	return TradingConfig{
		Enabled:             false,
		DefaultLeverage:     10,
		MaxLeverage:         20,
		DefaultRiskPct:      1.0,
		MaxPositionsPerUser: 5,
		MaxTotalPositions:   50,
		MinAccountBalance:   100,
		EntryOrderType:      "Limit",
		ReduceOnly:          true,
		TimeInForce:         "GoodTillCancel",
		EntryOrderTTLSec:    300,
		TrailingStopPct:     0,
		BreakevenAt:         0,
		PartialCloseRR:      0,
		PartialClosePct:     0.5,
		MaxDailyDrawdownPct: 5.0,
		PublicAnalyticsOnly: true,
	}
}
