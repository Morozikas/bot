// Пакет charts — генерация торговых графиков.
// candles.go — генерация реалистичных свечей для контекста сигнала.
//
// Для LONG:
//   Свечи идут ВНИЗ к зоне OB (медвежий тренд) → sweep ниже OB.Bottom → разворот → вход
// Для SHORT:
//   Свечи идут ВВЕРХ к зоне OB (бычий тренд) → sweep выше OB.Top → разворот → вход
//
// FVG формируется АВТОМАТИЧЕСКИ в момент импульса после свипа.
// OB зона соответствует последней противоположной свече перед импульсом.
package charts

import "math/rand"

// Candle — одна OHLC свеча с объёмом.
type Candle struct {
	Open, High, Low, Close float64
	Volume                 float64
}

// GenerateCandles строит серию свечей которые:
// 1. Трендово движутся к зоне OB
// 2. Входят в OB (последняя свеча OB = наш Order Block)
// 3. Делают Liquidity Sweep за OB
// 4. Разворачиваются импульсно (здесь формируется FVG)
// 5. Откатываются к зоне входа (текущая цена)
func GenerateCandles(data *SignalChartData, count int) []Candle {
	rng := rand.New(rand.NewSource(int64(len(data.Symbol)) + 42))
	candles := make([]Candle, 0, count)

	isShort := data.Direction == "Short"

	// Определяем ключевые уровни
	obHigh := data.OBHigh
	obLow := data.OBLow
	if obHigh <= obLow {
		obHigh = data.EntryIdeal * 1.01
		obLow = data.EntryIdeal * 0.99
	}

	// Откуда начинаем (противоположная сторона от OB)
	var startPrice float64
	if isShort {
		// Шорт: начинаем ниже OB, растём к нему
		startPrice = data.Price * (1.0 - 0.008 - rng.Float64()*0.006)
	} else {
		// Лонг: начинаем выше OB, падаем к нему
		startPrice = data.Price * (1.0 + 0.008 + rng.Float64()*0.006)
	}

	// Фазы:
	// [0, trendEnd)     — тренд к OB
	// [trendEnd, obEnd) — вход в OB (сам OB = несколько свечей)
	// [obEnd]           — свип за OB (длинный фитиль)
	// [obEnd+1]         — мощный разворот (импульс = формирует FVG)
	// [obEnd+2, ...]    — коррекция к Entry зоне

	trendEnd := count * 55 / 100 // 55% — тренд
	obEnd := count * 70 / 100    // до 70% — в OB
	sweepIdx := obEnd             // свип
	impulseIdx := obEnd + 1      // разворот

	price := startPrice

	// Размер одного шага (ступенька к OB)
	var totalMove float64
	if isShort {
		totalMove = obHigh - startPrice
	} else {
		totalMove = startPrice - obLow
	}
	step := totalMove / float64(trendEnd)
	atr := abs64(data.EntryIdeal-data.StopLoss) * 0.4

	for i := 0; i < count; i++ {
		var c Candle
		noise := atr * (0.2 + rng.Float64()*0.5)

		switch {
		case i < trendEnd:
			// Трендовые свечи с чередованием
			correction := i%4 == 1 || i%7 == 3
			if isShort {
				if correction {
					c = makeCandle(rng, price, -noise*0.25, noise) // небольшой откат вниз
				} else {
					c = makeCandle(rng, price, step*1.1, noise*0.6) // рост к OB
				}
			} else {
				if correction {
					c = makeCandle(rng, price, noise*0.25, noise)
				} else {
					c = makeCandle(rng, price, -step*1.1, noise*0.6)
				}
			}

		case i >= trendEnd && i < obEnd:
			// Свечи внутри OB зоны — небольшие, нерешительные
			if isShort {
				// В OB для шорта: мелкие бычьи с верхними фитилями
				c = makeCandle(rng, price, noise*0.15, noise*0.4)
				c.High = obHigh + noise*0.1 // касаемся верхней границы OB
			} else {
				c = makeCandle(rng, price, -noise*0.15, noise*0.4)
				c.Low = obLow - noise*0.1
			}
			normalizeCandle(&c)

		case i == sweepIdx:
			// SWEEP — длинный фитиль за OB, тело маленькое (пробой и возврат)
			if isShort {
				c.Open = obHigh + noise*0.05
				c.Close = obHigh - noise*0.2    // закрылись ниже OB.High
				c.High = obHigh + noise*0.6     // шип выше OB
				c.Low = c.Close - noise*0.05
			} else {
				c.Open = obLow - noise*0.05
				c.Close = obLow + noise*0.2
				c.High = c.Close + noise*0.05
				c.Low = obLow - noise*0.6
			}
			normalizeCandle(&c)

		case i == impulseIdx:
			// ИМПУЛЬС после свипа — сильная разворотная свеча
			// Это формирует FVG: разрыв между предыдущей High и следующей Low
			if isShort {
				c.Open = price + noise*0.05
				c.Close = price - atr*1.8   // сильное падение
				c.High = c.Open + noise*0.1
				c.Low = c.Close - noise*0.2
			} else {
				c.Open = price - noise*0.05
				c.Close = price + atr*1.8
				c.Low = c.Open - noise*0.1
				c.High = c.Close + noise*0.2
			}
			normalizeCandle(&c)

		case i > impulseIdx && i < count-2:
			// Коррекция к Entry зоне
			target := data.EntryIdeal
			diff := target - price
			bias := diff * 0.35
			c = makeCandle(rng, price, bias, noise*0.4)

		default:
			// Последние свечи — консолидация у Entry
			if isShort {
				c = makeCandle(rng, price, -noise*0.05, noise*0.25)
			} else {
				c = makeCandle(rng, price, noise*0.05, noise*0.25)
			}
		}

		// Объём — высокий на свипе и импульсе
		c.Volume = 50 + rng.Float64()*80
		if i == sweepIdx || i == impulseIdx {
			c.Volume = 200 + rng.Float64()*150
		}

		candles = append(candles, c)
		price = c.Close
	}
	return candles
}

// makeCandle создаёт свечу с заданным смещением и шумом фитилей.
func makeCandle(rng *rand.Rand, price, bias, wickNoise float64) Candle {
	var c Candle
	c.Open = price + wickNoise*0.05*(rng.Float64()-0.5)
	c.Close = c.Open + bias + wickNoise*0.15*(rng.Float64()-0.5)

	// Фитили
	topWick := wickNoise * (0.1 + rng.Float64()*0.3)
	botWick := wickNoise * (0.1 + rng.Float64()*0.3)
	c.High = max64(c.Open, c.Close) + topWick
	c.Low = min64(c.Open, c.Close) - botWick
	normalizeCandle(&c)
	return c
}

func normalizeCandle(c *Candle) {
	if c.High < max64(c.Open, c.Close) {
		c.High = max64(c.Open, c.Close) * 1.0005
	}
	if c.Low > min64(c.Open, c.Close) {
		c.Low = min64(c.Open, c.Close) * 0.9995
	}
}

func max64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func min64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
