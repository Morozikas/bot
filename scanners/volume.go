// Пакет scanners — сканирование рынка.
// volume.go — фильтрация по объёму: MA20, относительный объём, Volume Spike.
package scanners

import "bybit-elite-signal/api"

// VolumeFilter проверяет объёмное подтверждение.
type VolumeFilter struct {
	MinRelVolume float64 // min current/MA20 (default 1.0)
}

// NewVolumeFilter создаёт фильтр.
func NewVolumeFilter(minRelVolume float64) *VolumeFilter {
	return &VolumeFilter{MinRelVolume: minRelVolume}
}

// IsAllowed возвращает true если объём выше порога.
func (f *VolumeFilter) IsAllowed(candles []api.Candle) bool {
	rel := RelativeVolume(candles)
	return rel >= f.MinRelVolume
}

// RelativeVolume вычисляет отношение текущего объёма к MA20.
func RelativeVolume(candles []api.Candle) float64 {
	if len(candles) < 21 {
		return 0
	}
	cur, _ := candles[len(candles)-1].Volume.Float64()
	sum := 0.0
	for _, c := range candles[len(candles)-21 : len(candles)-1] {
		v, _ := c.Volume.Float64()
		sum += v
	}
	ma20 := sum / 20
	if ma20 == 0 {
		return 0
	}
	return cur / ma20
}

// IsVolumeSpike возвращает true если объём > 3x MA20.
func IsVolumeSpike(candles []api.Candle) bool {
	return RelativeVolume(candles) >= 3.0
}
