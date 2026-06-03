// Пакет scanners — сканирование и ранжирование рынка.
// baseline.go — накопление исторических средних по объёму и OI для каждой монеты.
//
// Решает проблему абсолютных чисел: BTC всегда будет топ-1 по абсолютному объёму.
// С относительным подходом маленькая монета с объёмом в 5× выше среднего
// обгоняет BTC с обычным объёмом.
//
// История накапливается автоматически при каждом скане.
// Данные живут только в памяти — после рестарта начинается заново.
package scanners

import (
	"math"
	"sync"
)

// historyLen — количество последних наблюдений для расчёта среднего (~24 скана = 4 часа при 600 сек интервале).
const historyLen = 24

// Baseline хранит скользящие средние объёма и OI по каждому символу.
type Baseline struct {
	mu      sync.RWMutex
	volumes map[string][]float64 // последние N значений Volume24h (в USD)
	ois     map[string][]float64 // последние N значений OI (в USD)
}

// NewBaseline создаёт Baseline.
func NewBaseline() *Baseline {
	return &Baseline{
		volumes: make(map[string][]float64),
		ois:     make(map[string][]float64),
	}
}

// Update добавляет новые наблюдения для символа (вызывается после каждого GetTickers).
func (b *Baseline) Update(symbol string, volume24hUSD, oiUSD float64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.volumes[symbol] = appendCapped(b.volumes[symbol], volume24hUSD, historyLen)
	b.ois[symbol] = appendCapped(b.ois[symbol], oiUSD, historyLen)
}

// RelativeVolume возвращает текущий объём / средний объём за N наблюдений.
// Если истории нет (первые сканы) — возвращает 1.0 (нейтральное значение).
// Пример: 3.0 = объём в 3 раза выше обычного.
func (b *Baseline) RelativeVolume(symbol string, currentVol float64) float64 {
	b.mu.RLock()
	defer b.mu.RUnlock()

	hist := b.volumes[symbol]
	avg := average(hist)
	if avg <= 0 || len(hist) < 3 {
		return 1.0 // недостаточно истории — нейтральное значение
	}
	return currentVol / avg
}

// RelativeOI возвращает нормализованное изменение OI относительно исторического среднего.
// Положительное = OI выше обычного (рост интереса), отрицательное = ниже.
// Диапазон примерно от -2.0 до +5.0.
func (b *Baseline) RelativeOI(symbol string, currentOI float64) float64 {
	b.mu.RLock()
	defer b.mu.RUnlock()

	hist := b.ois[symbol]
	if len(hist) < 3 {
		return 0.0
	}
	avg := average(hist)
	if avg <= 0 {
		return 0.0
	}
	return (currentOI - avg) / avg // +0.5 = OI на 50% выше среднего
}

// HasHistory возвращает true если накоплено достаточно данных для надёжной оценки.
func (b *Baseline) HasHistory(symbol string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.volumes[symbol]) >= 3
}

// HistoryLen возвращает количество накопленных наблюдений.
func (b *Baseline) HistoryLen(symbol string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.volumes[symbol])
}

// ---- утилиты ----

func appendCapped(slice []float64, val float64, cap int) []float64 {
	slice = append(slice, val)
	if len(slice) > cap {
		slice = slice[len(slice)-cap:]
	}
	return slice
}

func average(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

// stdDev возвращает стандартное отклонение (для будущего Z-score).
func stdDev(vals []float64) float64 {
	if len(vals) < 2 {
		return 0
	}
	avg := average(vals)
	variance := 0.0
	for _, v := range vals {
		d := v - avg
		variance += d * d
	}
	return math.Sqrt(variance / float64(len(vals)-1))
}

var _ = stdDev // используется потенциально в будущем
