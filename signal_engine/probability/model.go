// Пакет probability — вероятностная модель сигнала.
// model.go — оценка вероятности выигрыша на основе Score и исторической статистики.
package probability

// Estimate вычисляет вероятность успеха сигнала (%) из Score (0-100).
// Формула: линейная интерполяция 40%+Score*0.6 с поправкой на RR.
func Estimate(score, rr float64) float64 {
	base := score*0.6 + 40
	// Высокий RR немного снижает "вероятность", так как цель дальше
	if rr >= 7 {
		base -= 3
	} else if rr >= 5 {
		base -= 1
	}
	if base > 99 {
		return 99
	}
	if base < 0 {
		return 0
	}
	return base
}
