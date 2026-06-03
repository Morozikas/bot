// Пакет volume — анализ объёма.
// hvn.go — High Volume Node: зона максимального скопления объёма (поддержка/сопротивление).
// lvn.go — Low Volume Node: зона минимального объёма (быстрое прохождение цены).
package volume

// HVNInfo — информация о High Volume Node.
type HVNInfo struct {
	Price    float64
	Volume   float64
	IsAbove  bool // выше текущей цены (сопротивление)
	IsBelow  bool // ниже (поддержка)
}

// NearestHVNAbove возвращает ближайший HVN выше текущей цены (цель TP для лонгов).
func NearestHVNAbove(p *Profile, price float64) float64 {
	best := 0.0
	for _, h := range p.HVNs {
		if h > price && (best == 0 || h < best) {
			best = h
		}
	}
	return best
}

// NearestHVNBelow возвращает ближайший HVN ниже текущей цены (цель TP для шортов).
func NearestHVNBelow(p *Profile, price float64) float64 {
	best := 0.0
	for _, h := range p.HVNs {
		if h < price && (best == 0 || h > best) {
			best = h
		}
	}
	return best
}

// NearestLVNAbove возвращает ближайший LVN выше цены (зона ускорения).
func NearestLVNAbove(p *Profile, price float64) float64 {
	best := 0.0
	for _, l := range p.LVNs {
		if l > price && (best == 0 || l < best) {
			best = l
		}
	}
	return best
}
