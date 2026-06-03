// Пакет scoring — оценка сигналов.
// session.go — весовой коэффициент торговой сессии (Лондон > Нью-Йорк > Азия).
package scoring

import (
	"time"

	"bybit-elite-signal/config"
)

// SessionWeight возвращает множитель для текущей торговой сессии UTC.
func SessionWeight(cfg *config.SignalConfig) float64 {
	hour := time.Now().UTC().Hour()
	switch {
	case hour >= 13 && hour < 17: // Overlap London/NY
		return cfg.OverlapSessionWeight
	case hour >= 8 && hour < 13: // London
		return cfg.LondonSessionWeight
	case hour >= 17 && hour < 22: // New York
		return cfg.NYSessionWeight
	case hour >= 0 && hour < 8: // Asia
		return cfg.AsiaSessionWeight
	default:
		return cfg.OffHoursWeight
	}
}
