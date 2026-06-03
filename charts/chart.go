// Пакет charts — генерация торговых графиков в стиле TradingView.
// chart.go — рендерер: тёмный фон, свечи OHLC, OB/FVG зоны, TP/SL полосы, ось цен справа.
package charts

import (
	"bytes"
	"fmt"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/fogleman/gg"
)

// Generate создаёт PNG и возвращает байты.
func Generate(data *SignalChartData, cfg ChartConfig) ([]byte, error) {
	W := float64(cfg.Width)
	H := float64(cfg.Height)

	dc := gg.NewContext(cfg.Width, cfg.Height)

	// Генерируем свечи
	candles := GenerateCandles(data, cfg.CandleCount)

	// ── Диапазон цен ──────────────────────────────────────────────────────────
	priceMin, priceMax := priceRange(data, candles)
	pad := (priceMax - priceMin) * 0.10
	priceMin -= pad
	priceMax += pad

	// ── Разметка пространства ─────────────────────────────────────────────────
	// Слева — свечи + зоны (80% ширины), справа — ось цен (20%)
	priceAxisW := 90.0       // ширина панели цен справа
	chartX0 := 0.0           // левый край свечей
	chartX1 := W - priceAxisW // правый край свечей
	chartW := chartX1 - chartX0
	topPad := 20.0
	botPad := 30.0
	chartY0 := topPad
	chartY1 := H - botPad
	chartH := chartY1 - chartY0

	toY := func(p float64) float64 {
		ratio := (p - priceMin) / (priceMax - priceMin)
		return chartY1 - ratio*chartH
	}

	slotW := chartW / float64(cfg.CandleCount)
	bodyW := slotW * cfg.CandleWidth

	// ── 1. Фон ────────────────────────────────────────────────────────────────
	dc.SetColor(cfg.BgColor)
	dc.Clear()

	// ── 2. Горизонтальная сетка ───────────────────────────────────────────────
	steps := 8
	for i := 0; i <= steps; i++ {
		p := priceMin + (priceMax-priceMin)*float64(i)/float64(steps)
		y := toY(p)
		dc.SetColor(cfg.GridColor)
		dc.SetLineWidth(0.5)
		dc.DrawLine(chartX0, y, chartX1, y)
		dc.Stroke()
	}

	// ── 3. OB зона (полная ширина, серая полупрозрачная) ────────────────────
	obTop := toY(data.OBHigh)
	obBot := toY(data.OBLow)
	drawZone(dc, chartX0, obTop, chartX1-chartX0, obBot-obTop,
		color.RGBA{120, 120, 145, 35}, color.RGBA{160, 160, 185, 90}, false)
	// Метка OB внутри зоны
	setFontSize(dc, cfg.FontPath, 11)
	dc.SetColor(color.RGBA{200, 200, 215, 200})
	dc.DrawString(cfg.OBLabel, chartX0+8, obTop+14)

	// ── 4. FVG зона (полная ширина, фиолетовая) ──────────────────────────────
	if data.FVGLow > 0 && data.FVGHigh > 0 {
		fvgTop := toY(data.FVGHigh)
		fvgBot := toY(data.FVGLow)
		if fvgBot > fvgTop {
			drawZone(dc, chartX0, fvgTop, chartX1-chartX0, fvgBot-fvgTop,
				color.RGBA{90, 55, 160, 45}, color.RGBA{140, 90, 210, 100}, false)
			setFontSize(dc, cfg.FontPath, 11)
			dc.SetColor(color.RGBA{180, 140, 230, 210})
			dc.DrawString(cfg.FVGLabel, chartX0+8, fvgTop+14)
		}
	}

	// ── 5. TP зона (зелёная полоса на правой части) ───────────────────────────
	tpY := toY(data.TakeProfit)
	entryY := toY(data.EntryIdeal)
	slY := toY(data.StopLoss)

	if data.Direction == "Long" {
		// Зелёная зона: entry → TP (прибыль)
		dc.SetColor(color.RGBA{0, 60, 40, 55})
		dc.DrawRectangle(chartX0, tpY, chartX1-chartX0, entryY-tpY)
		dc.Fill()
		// Красная зона: SL → entry (риск)
		dc.SetColor(color.RGBA{80, 15, 15, 55})
		dc.DrawRectangle(chartX0, entryY, chartX1-chartX0, slY-entryY)
		dc.Fill()
	} else {
		// Зелёная зона: entry → TP (вниз для шорта)
		dc.SetColor(color.RGBA{0, 60, 40, 55})
		dc.DrawRectangle(chartX0, entryY, chartX1-chartX0, tpY-entryY)
		dc.Fill()
		// Красная зона: entry → SL (вверх для шорта)
		dc.SetColor(color.RGBA{80, 15, 15, 55})
		dc.DrawRectangle(chartX0, slY, chartX1-chartX0, entryY-slY)
		dc.Fill()
	}

	// ── 6. Горизонтальные линии TP, Entry, SL ────────────────────────────────
	// TP — зелёная
	drawHLine(dc, chartX0, tpY, chartX1, cfg.TPColor, 1.5, nil)
	// Entry — жёлтая пунктирная
	drawHLine(dc, chartX0, entryY, chartX1, cfg.EntryZoneBorder, 1, []float64{6, 4})
	// SL — красная
	drawHLine(dc, chartX0, slY, chartX1, cfg.SLColor, 1.5, nil)

	// Зона входа (entry_low..entry_high, жёлтая, полупрозрачная)
	ezTop := toY(data.EntryZoneHigh)
	ezBot := toY(data.EntryZoneLow)
	drawZone(dc, chartX0, ezTop, chartX1-chartX0, ezBot-ezTop,
		color.RGBA{180, 160, 20, 25}, color.RGBA{200, 180, 40, 70}, true)

	// ── 7. Свечи ─────────────────────────────────────────────────────────────
	for i, c := range candles {
		cx := chartX0 + float64(i)*slotW + slotW*0.5
		bullish := c.Close >= c.Open
		col := cfg.BullColor
		if !bullish {
			col = cfg.BearColor
		}

		// Фитиль
		dc.SetColor(col)
		dc.SetLineWidth(1)
		dc.DrawLine(cx, toY(c.High), cx, toY(c.Low))
		dc.Stroke()

		// Тело
		bodyTop := math.Min(toY(c.Open), toY(c.Close))
		bodyH := math.Abs(toY(c.Close) - toY(c.Open))
		if bodyH < 1.5 {
			bodyH = 1.5 // doji — минимальная высота
		}
		dc.SetColor(col)
		dc.DrawRectangle(cx-bodyW/2, bodyTop, bodyW, bodyH)
		dc.Fill()
	}

	// ── 8. Текущая цена (пунктир) ────────────────────────────────────────────
	curY := toY(data.Price)
	drawHLine(dc, chartX0, curY, chartX1, color.RGBA{100, 120, 140, 180}, 1, []float64{3, 5})

	// ── 9. Ось цен справа ────────────────────────────────────────────────────
	// Фон оси
	dc.SetColor(color.RGBA{20, 24, 36, 255})
	dc.DrawRectangle(chartX1, 0, priceAxisW, H)
	dc.Fill()

	// Разделитель
	dc.SetColor(color.RGBA{50, 55, 70, 255})
	dc.SetLineWidth(1)
	dc.DrawLine(chartX1, chartY0, chartX1, chartY1)
	dc.Stroke()

	// Цены на сетке
	setFontSize(dc, cfg.FontPath, 10)
	for i := 0; i <= steps; i++ {
		p := priceMin + (priceMax-priceMin)*float64(i)/float64(steps)
		y := toY(p)
		if y < chartY0+8 || y > chartY1-4 {
			continue
		}
		// Тик
		dc.SetColor(color.RGBA{70, 75, 90, 200})
		dc.DrawLine(chartX1, y, chartX1+4, y)
		dc.Stroke()
		// Метка
		dc.SetColor(color.RGBA{160, 165, 180, 200})
		label := fmtPrice(p)
		dc.DrawStringAnchored(label, chartX1+priceAxisW-4, y+1, 1.0, 0.5)
	}

	// ── 10. Метки TP, SL, Entry, текущей цены на оси ────────────────────────
	// Залитые теги: каждый уровень — цветной прямоугольник с ценой
	drawFilledTag(dc, chartX1, tpY, priceAxisW, cfg.TPColor,
		cfg.TPLabel+" "+fmtPrice(data.TakeProfit), cfg.FontPath)
	drawFilledTag(dc, chartX1, slY, priceAxisW, cfg.SLColor,
		cfg.SLLabel+" "+fmtPrice(data.StopLoss), cfg.FontPath)
	drawFilledTag(dc, chartX1, entryY, priceAxisW,
		color.RGBA{180, 150, 30, 210}, fmtPrice(data.EntryIdeal), cfg.FontPath)
	// Текущая цена — синий залитый тег
	drawPriceTagFilled(dc, chartX1, curY, priceAxisW,
		color.RGBA{30, 110, 175, 230}, fmtPrice(data.Price), cfg.FontPath)

	// ── 11. Информационная панель (левый верхний угол) ────────────────────────
	drawInfoPanel(dc, cfg, data)

	// ── 12. Временная метка ───────────────────────────────────────────────────
	setFontSize(dc, cfg.FontPath, 9)
	dc.SetColor(cfg.WatermarkColor)
	ts := fmt.Sprintf("%s · %s UTC", cfg.WatermarkText, time.Now().UTC().Format("02.01.2006 15:04"))
	dc.DrawStringAnchored(ts, chartX1-4, H-8, 1.0, 0.0)

	// ── Экспорт ───────────────────────────────────────────────────────────────
	var buf bytes.Buffer
	if err := dc.EncodePNG(&buf); err != nil {
		return nil, fmt.Errorf("encode PNG: %w", err)
	}
	if cfg.SaveDir != "" {
		_ = os.MkdirAll(cfg.SaveDir, 0755)
		fname := filepath.Join(cfg.SaveDir,
			fmt.Sprintf("%s_%s_%s.png", data.Symbol, data.Direction,
				time.Now().UTC().Format("20060102_150405")))
		_ = os.WriteFile(fname, buf.Bytes(), 0644)
		// Удаляем старые файлы — оставляем не более 10
		cleanOldPNGs(cfg.SaveDir, 10)
	}
	return buf.Bytes(), nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func drawZone(dc *gg.Context, x, y, w, h float64, fill, border color.RGBA, dashed bool) {
	if w <= 0 || math.Abs(h) < 1 {
		return
	}
	if h < 0 {
		y += h
		h = -h
	}
	dc.SetColor(fill)
	dc.DrawRectangle(x, y, w, h)
	dc.Fill()
	if border.A > 0 {
		dc.SetColor(border)
		dc.SetLineWidth(1)
		if dashed {
			dc.SetDash(5, 4)
		} else {
			dc.SetDash()
		}
		dc.DrawRectangle(x, y, w, h)
		dc.Stroke()
		dc.SetDash()
	}
}

func drawHLine(dc *gg.Context, x1, y, x2 float64, col color.RGBA, width float64, dash []float64) {
	dc.SetColor(col)
	dc.SetLineWidth(width)
	if len(dash) > 0 {
		dc.SetDash(dash...)
	} else {
		dc.SetDash()
	}
	dc.DrawLine(x1, y, x2, y)
	dc.Stroke()
	dc.SetDash()
}

// drawPriceTag рисует метку цены (линия + текст, без заливки).
func drawPriceTag(dc *gg.Context, axisX, y, axisW float64, col color.RGBA, label, fontPath string) {
	dc.SetColor(col)
	dc.SetLineWidth(1)
	dc.DrawLine(axisX, y, axisX+6, y)
	dc.Stroke()
	setFontSize(dc, fontPath, 10)
	dc.SetColor(col)
	dc.DrawStringAnchored(label, axisX+axisW-4, y+1, 1.0, 0.5)
}

// drawFilledTag — залитый прямоугольник-тег с текстом (TP/SL/Entry).
func drawFilledTag(dc *gg.Context, axisX, y, axisW float64, col color.RGBA, label, fontPath string) {
	// Маленький треугольник-стрелка слева
	dc.SetColor(col)
	dc.SetLineWidth(1.5)
	dc.DrawLine(axisX, y, axisX+6, y)
	dc.Stroke()
	// Прямоугольник
	tagH := 16.0
	dc.SetColor(col)
	dc.DrawRectangle(axisX+2, y-tagH/2, axisW-4, tagH)
	dc.Fill()
	// Текст
	setFontSize(dc, fontPath, 10)
	// Вычисляем яркость цвета для выбора белого/чёрного текста
	brightness := float64(col.R)*0.299 + float64(col.G)*0.587 + float64(col.B)*0.114
	if brightness > 128 {
		dc.SetColor(color.RGBA{20, 20, 20, 255})
	} else {
		dc.SetColor(color.RGBA{240, 240, 240, 255})
	}
	dc.DrawStringAnchored(label, axisX+axisW/2+1, y+1, 0.5, 0.5)
}

// drawPriceTagFilled рисует залитый прямоугольник с ценой (текущая цена).
func drawPriceTagFilled(dc *gg.Context, axisX, y, axisW float64, bg color.RGBA, label, fontPath string) {
	dc.SetColor(bg)
	dc.DrawRectangle(axisX+2, y-9, axisW-4, 18)
	dc.Fill()
	setFontSize(dc, fontPath, 10)
	dc.SetColor(color.RGBA{255, 255, 255, 230})
	dc.DrawStringAnchored(label, axisX+axisW/2+1, y+1, 0.5, 0.5)
}

func drawInfoPanel(dc *gg.Context, cfg ChartConfig, data *SignalChartData) {
	px, py := 10.0, 10.0
	lh := 16.0
	pw := 230.0

	lines := 10
	if data.Deadline != "" {
		lines++
	}
	if data.Catalyst != "" {
		lines += 2
	}
	ph := float64(lines)*lh + 14

	dc.SetColor(cfg.PanelBg)
	dc.DrawRoundedRectangle(px, py, pw, ph, 5)
	dc.Fill()

	y := py + lh
	dir := "📈"
	if data.Direction == "Short" {
		dir = "📉"
	}
	setFontSize(dc, cfg.FontPath, 12)
	dc.SetColor(cfg.PanelText)
	dc.DrawString(fmt.Sprintf("%s %s · %s · %s", dir, data.Symbol, data.Direction, data.Timeframe), px+6, y)
	y += lh * 0.3

	sep := func() {
		dc.SetColor(color.RGBA{70, 75, 90, 200})
		dc.DrawLine(px+4, y, px+pw-4, y)
		dc.Stroke()
		y += lh * 0.5
	}
	sep()

	row := func(label, value string, vc color.RGBA) {
		setFontSize(dc, cfg.FontPath, 10)
		dc.SetColor(cfg.PanelLabel)
		dc.DrawString(label, px+6, y)
		dc.SetColor(vc)
		// Выравнивание значений по одной колонке
		dc.DrawStringAnchored(value, px+pw-8, y, 1.0, 0.0)
		y += lh
	}

	white := cfg.PanelText
	slPct := math.Abs(data.EntryIdeal-data.StopLoss) / data.EntryIdeal * 100
	tpPct := math.Abs(data.TakeProfit-data.EntryIdeal) / data.EntryIdeal * 100

	row("Score:", fmt.Sprintf("%.0f/100", data.Score), white)
	if data.Mode != "" {
		row("Режим:", data.Mode, white)
	}
	sep()
	row("Вход:", fmtPrice(data.EntryIdeal), cfg.EntryZoneBorder)
	row("Зона:", fmt.Sprintf("%s – %s", fmtPrice(data.EntryZoneLow), fmtPrice(data.EntryZoneHigh)), cfg.EntryZoneBorder)
	row(cfg.SLLabel+":", fmt.Sprintf("%s (−%.2f%%)", fmtPrice(data.StopLoss), slPct), cfg.SLColor)
	row(cfg.TPLabel+":", fmt.Sprintf("%s (+%.2f%%)", fmtPrice(data.TakeProfit), tpPct), cfg.TPColor)
	row("RR:", fmt.Sprintf("1 : %.1f", data.RR), white)
	if data.Deadline != "" {
		sep()
		row("⏰", data.Deadline, color.RGBA{220, 190, 60, 255})
	}
	if data.Catalyst != "" {
		sep()
		setFontSize(dc, cfg.FontPath, 9)
		dc.SetColor(cfg.PanelLabel)
		dc.DrawString("Setup:", px+6, y)
		y += lh * 0.9
		dc.SetColor(white)
		drawWrapped(dc, data.Catalyst, px+6, y, pw-12, lh*0.85)
	}
}

func drawWrapped(dc *gg.Context, text string, x, y, maxW, lh float64) {
	line := ""
	for _, r := range text {
		test := line + string(r)
		w, _ := dc.MeasureString(test)
		if w > maxW && line != "" {
			dc.DrawString(line, x, y)
			y += lh
			line = string(r)
		} else {
			line = test
		}
	}
	if line != "" {
		dc.DrawString(line, x, y)
	}
}

func setFontSize(dc *gg.Context, fontPath string, size float64) {
	if fontPath != "" {
		if err := dc.LoadFontFace(fontPath, size); err == nil {
			return
		}
	}
	_ = dc.LoadFontFace("", size)
}

func priceRange(data *SignalChartData, candles []Candle) (min, max float64) {
	min = data.StopLoss * 0.999
	max = data.TakeProfit * 1.001
	for _, c := range candles {
		if c.Low < min {
			min = c.Low
		}
		if c.High > max {
			max = c.High
		}
	}
	return
}

// cleanOldPNGs удаляет старые PNG из директории, оставляя maxKeep последних.
func cleanOldPNGs(dir string, maxKeep int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var pngs []string
	for _, e := range entries {
		if !e.IsDir() && len(e.Name()) > 4 && e.Name()[len(e.Name())-4:] == ".png" {
			pngs = append(pngs, filepath.Join(dir, e.Name()))
		}
	}
	// ReadDir возвращает в алфавитном порядке — имена содержат timestamp, поэтому
	// алфавитный порядок = хронологический. Удаляем самые старые.
	for len(pngs) > maxKeep {
		_ = os.Remove(pngs[0])
		pngs = pngs[1:]
	}
}

func fmtPrice(p float64) string {
	if p >= 10000 {
		return fmt.Sprintf("%.1f", p)
	}
	if p >= 100 {
		return fmt.Sprintf("%.2f", p)
	}
	return fmt.Sprintf("%.4f", p)
}
