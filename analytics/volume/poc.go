// Пакет volume — анализ объёма.
// poc.go — Point of Control: уровень с максимальным торговым объёмом.
package volume

// NakedPOC — POC, который ещё не был ретестирован ценой после формирования.
type NakedPOC struct {
	Price    float64
	Session  string // "Daily" | "Weekly" | "Session"
	Retested bool
}

// FindNakedPOC ищет незатронутые POC из переданных профилей.
func FindNakedPOC(profiles []*Profile, currentPrice float64) []NakedPOC {
	result := make([]NakedPOC, 0)
	for _, p := range profiles {
		if p == nil {
			continue
		}
		// POC считается Naked если цена ещё не вернулась к нему
		touched := currentPrice >= p.POC*0.9995 && currentPrice <= p.POC*1.0005
		result = append(result, NakedPOC{
			Price: p.POC, Retested: touched,
		})
	}
	return result
}
