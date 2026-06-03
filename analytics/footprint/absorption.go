// Пакет footprint — анализ ордер-флоу.
// absorption.go — абсорбция: поглощение агрессивных покупок или продаж крупным игроком.
package footprint

// Absorption — обнаруженная абсорбция объёма.
type Absorption struct {
	Type      string  // "BuyerAbsorption" | "SellerAbsorption"
	Price     float64
	BidVol    float64
	AskVol    float64
	Intensity float64 // 0-100
}

// DetectAbsorption ищет свечи с абсорбцией (большой объём, маленькое тело).
func DetectAbsorption(dc []DeltaCandle) []Absorption {
	result := make([]Absorption, 0)
	if len(dc) == 0 {
		return result
	}
	last := dc[len(dc)-1]
	body := last.Close - last.Open
	if body < 0 {
		body = -body
	}
	// Абсорбция: объём обеих сторон значительный, но тело маленькое
	totalVol := last.BidVol + last.AskVol
	if totalVol <= 0 {
		return result
	}
	bodyRatio := body / ((last.Close + last.Open) / 2) * 100

	if bodyRatio < 0.3 && totalVol > 0 {
		t := "BuyerAbsorption"
		if last.AskVol > last.BidVol {
			t = "SellerAbsorption"
		}
		result = append(result, Absorption{
			Type: t, Price: last.Close,
			BidVol: last.BidVol, AskVol: last.AskVol,
			Intensity: 100 - bodyRatio*100,
		})
	}
	return result
}
