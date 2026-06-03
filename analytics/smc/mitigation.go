// Пакет smc — Smart Money Concepts.
// mitigation.go — Mitigation Block: частично отработанный OB.
package smc

// MitigationBlock — Order Block с частичным снятием ликвидности.
type MitigationBlock struct {
	Type      OBType
	Top       float64
	Bottom    float64
	Mitigated float64 // % снятого объёма
	Quality   float64
}

// DetectMitigationBlocks находит частично отработанные OB.
func DetectMitigationBlocks(obs []OrderBlock) []MitigationBlock {
	result := make([]MitigationBlock, 0)
	for _, ob := range obs {
		if ob.Freshness > 0 && ob.Freshness < 1.0 {
			mitigated := (1.0 - ob.Freshness) * 100
			result = append(result, MitigationBlock{
				Type:      ob.Type,
				Top:       ob.Top,
				Bottom:    ob.Bottom,
				Mitigated: mitigated,
				Quality:   ob.Quality * ob.Freshness,
			})
		}
	}
	return result
}
