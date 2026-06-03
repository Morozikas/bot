// Пакет liquidity — анализ ликвидности.
// map.go — полная карта ликвидности: buy side, sell side, пулы, кластеры ликвидаций.
package liquidity

import (
	"bybit-elite-signal/api"
	"bybit-elite-signal/config"
)

// LiquidityMap — полная карта ликвидности для символа.
type LiquidityMap struct {
	EqualHighs          []LiquidityLevel
	EqualLows           []LiquidityLevel
	BuySide             []LiquidityLevel // выше цены
	SellSide            []LiquidityLevel // ниже цены
	Pools               []Pool
	Sweep               *SweepResult
	StopHunts           []StopHunt
	LiquidationClusters []LiquidationCluster
	HasLiquidity        bool
}

// LiquidationCluster — зона принудительных ликвидаций.
type LiquidationCluster struct {
	Price    float64
	Side     string
	Notional float64
}

// Build строит полную карту ликвидности.
func Build(candles []api.Candle, liquidations []api.LiquidationRecord, cfg *config.SignalConfig) *LiquidityMap {
	m := &LiquidityMap{}
	if len(candles) < 10 {
		return m
	}
	lastClose, _ := candles[len(candles)-1].Close.Float64()
	tol := cfg.EqualLevelsTolerance

	m.EqualHighs = EqualHighs(candles, tol)
	m.EqualLows = EqualLows(candles, tol)

	for _, l := range m.EqualHighs {
		if l.Price > lastClose {
			m.BuySide = append(m.BuySide, l)
		}
	}
	for _, l := range m.EqualLows {
		if l.Price < lastClose {
			m.SellSide = append(m.SellSide, l)
		}
	}

	for _, l := range append(m.BuySide, m.SellSide...) {
		side := "BuySide"
		if l.Price < lastClose {
			side = "SellSide"
		}
		m.Pools = append(m.Pools, Pool{
			High: l.Price * 1.001, Low: l.Price * 0.999,
			Strength: l.Strength, Type: side,
		})
	}

	m.Sweep = DetectSweep(candles, m.EqualHighs, m.EqualLows)
	m.StopHunts = DetectStopHunts(candles)
	m.LiquidationClusters = clusterLiquidations(liquidations, lastClose)
	m.HasLiquidity = len(m.BuySide)+len(m.SellSide) > 0
	return m
}

func clusterLiquidations(records []api.LiquidationRecord, currentPrice float64) []LiquidationCluster {
	if len(records) == 0 || currentPrice == 0 {
		return nil
	}
	buckets := make(map[int]float64)
	for _, r := range records {
		p, _ := r.Price.Float64()
		q, _ := r.Qty.Float64()
		buckets[int(p/(currentPrice*0.01))] += p * q
	}
	result := make([]LiquidationCluster, 0)
	for bucket, notional := range buckets {
		if notional < 100_000 {
			continue
		}
		price := float64(bucket) * currentPrice * 0.01
		side := "Long"
		if price > currentPrice {
			side = "Short"
		}
		result = append(result, LiquidationCluster{Price: price, Side: side, Notional: notional})
	}
	return result
}
