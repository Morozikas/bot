// Пакет candles — управление OHLCV-свечами.
// timeframes.go — конвертация и валидация таймфреймов Bybit.
package candles

// TFMinutes конвертирует код таймфрейма в минуты.
func TFMinutes(tf string) int {
	m := map[string]int{
		"1": 1, "3": 3, "5": 5, "15": 15, "30": 30,
		"60": 60, "120": 120, "240": 240, "360": 360, "720": 720,
		"D": 1440, "W": 10080, "M": 43200,
	}
	if v, ok := m[tf]; ok {
		return v
	}
	return 0
}

// IsHigherTF возвращает true, если a старше b.
func IsHigherTF(a, b string) bool {
	return TFMinutes(a) > TFMinutes(b)
}

// Higher возвращает следующий старший таймфрейм.
func Higher(tf string) string {
	chain := []string{"1", "5", "15", "60", "240", "D", "W"}
	for i, t := range chain {
		if t == tf && i+1 < len(chain) {
			return chain[i+1]
		}
	}
	return tf
}
