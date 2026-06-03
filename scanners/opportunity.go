// Пакет scanners — сканирование рынка.
// opportunity.go — финальный фильтр возможностей: комбинирует ранжирование и фильтры.
package scanners

import "bybit-elite-signal/api"

// Opportunity — символ с оценкой и флагами фильтров.
type Opportunity struct {
	SymbolScore
	VolumeOK    bool
	VolatilityOK bool
}

// FilterOpportunities применяет объёмный и волатильный фильтр к топу символов.
func FilterOpportunities(
	scores []SymbolScore,
	candlesMap map[string][]api.Candle,
	volFilter *VolumeFilter,
	atrFilter *VolatilityFilter,
) []Opportunity {
	result := make([]Opportunity, 0, len(scores))
	for _, s := range scores {
		candles := candlesMap[s.Symbol]
		opp := Opportunity{
			SymbolScore:  s,
			VolumeOK:     volFilter.IsAllowed(candles),
			VolatilityOK: atrFilter.IsAllowed(candles),
		}
		if opp.VolumeOK && opp.VolatilityOK {
			result = append(result, opp)
		}
	}
	return result
}
