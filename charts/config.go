// Пакет charts — генерация торговых графиков-сигналов в стиле TradingView.
// config.go — все визуальные параметры графика.
//
// Настройки загружаются из ОТДЕЛЬНОГО файла .env.chart
// (не из основного .env — чтобы не засорять его).
// Все параметры можно менять без перекомпиляции.
package charts

import (
	"bufio"
	"image/color"
	"os"
	"strconv"
	"strings"
)

const envChartFile = ".env.chart"

// ChartConfig — полная конфигурация внешнего вида графика.
type ChartConfig struct {
	Width  int
	Height int

	BgColor   color.RGBA
	GridColor color.RGBA

	CandleCount int
	BullColor   color.RGBA
	BearColor   color.RGBA
	CandleWidth float64

	OBFill    color.RGBA
	OBBorder  color.RGBA
	OBLabel   string

	FVGFill   color.RGBA
	FVGBorder color.RGBA
	FVGLabel  string

	ChannelColor color.RGBA
	ChannelFill  color.RGBA

	SRColor color.RGBA

	EntryZoneFill   color.RGBA
	EntryZoneBorder color.RGBA
	EntryLabel      string

	SLColor color.RGBA
	SLFill  color.RGBA
	SLLabel string

	TPColor color.RGBA
	TPFill  color.RGBA
	TPLabel string

	ArrowColor color.RGBA

	PanelBg    color.RGBA
	PanelText  color.RGBA
	PanelLabel color.RGBA

	CurrentPriceColor color.RGBA

	WatermarkText  string
	WatermarkColor color.RGBA

	FontPath string
	SaveDir  string
}

// DefaultConfig возвращает конфигурацию по умолчанию.
func DefaultConfig() ChartConfig {
	return ChartConfig{
		Width: 1280, Height: 720,

		BgColor:   hex("#131722"), GridColor: hex("#1e222d"),

		CandleCount: 25, BullColor: hex("#26a69a"), BearColor: hex("#ef5350"), CandleWidth: 0.65,

		OBFill: rgba(128, 0, 128, 64), OBBorder: hex("#9c27b0"), OBLabel: "OB",

		FVGFill: rgba(0, 128, 0, 51), FVGBorder: hex("#4caf50"), FVGLabel: "FVG",

		ChannelColor: hex("#2196f3"), ChannelFill: rgba(33, 150, 243, 13),

		SRColor: hex("#546e7a"),

		EntryZoneFill: rgba(255, 235, 59, 38), EntryZoneBorder: hex("#ffeb3b"), EntryLabel: "ВХОД",

		SLColor: hex("#f44336"), SLFill: rgba(244, 67, 54, 20), SLLabel: "СЛ",

		TPColor: hex("#4caf50"), TPFill: rgba(76, 175, 80, 20), TPLabel: "ТП",

		ArrowColor: hex("#2196f3"),

		PanelBg:    rgba(0, 0, 0, 179),
		PanelText:  hex("#ffffff"),
		PanelLabel: hex("#9e9e9e"),

		CurrentPriceColor: hex("#546e7a"),

		WatermarkText:  "Bybit Futures",
		WatermarkColor: hex("#37474f"),

		SaveDir: "./charts/output",
	}
}

// LoadConfig читает .env.chart и возвращает конфигурацию с переопределёнными значениями.
func LoadConfig() ChartConfig {
	cfg := DefaultConfig()
	env := readEnvFile(envChartFile)

	applyString := func(key string, dest *string) {
		if v, ok := env[key]; ok && v != "" {
			*dest = v
		}
	}
	applyInt := func(key string, dest *int) {
		if v, ok := env[key]; ok {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				*dest = n
			}
		}
	}
	applyFloat := func(key string, dest *float64) {
		if v, ok := env[key]; ok {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				*dest = f
			}
		}
	}
	applyColor := func(key string, dest *color.RGBA) {
		if v, ok := env[key]; ok && strings.HasPrefix(v, "#") {
			*dest = hex(v)
		}
	}
	applyAlpha := func(rKey, gKey, bKey, aKey string, dest *color.RGBA) {
		r, g, b, a := dest.R, dest.G, dest.B, dest.A
		if v, ok := env[rKey]; ok {
			if n, err := strconv.ParseUint(v, 10, 8); err == nil {
				r = uint8(n)
			}
		}
		if v, ok := env[gKey]; ok {
			if n, err := strconv.ParseUint(v, 10, 8); err == nil {
				g = uint8(n)
			}
		}
		if v, ok := env[bKey]; ok {
			if n, err := strconv.ParseUint(v, 10, 8); err == nil {
				b = uint8(n)
			}
		}
		if v, ok := env[aKey]; ok {
			if n, err := strconv.ParseUint(v, 10, 8); err == nil {
				a = uint8(n)
			}
		}
		*dest = color.RGBA{r, g, b, a}
	}

	applyInt("CHART_WIDTH", &cfg.Width)
	applyInt("CHART_HEIGHT", &cfg.Height)
	applyColor("CHART_BG", &cfg.BgColor)
	applyColor("CHART_GRID", &cfg.GridColor)
	applyInt("CHART_CANDLE_COUNT", &cfg.CandleCount)
	applyColor("CHART_BULL_COLOR", &cfg.BullColor)
	applyColor("CHART_BEAR_COLOR", &cfg.BearColor)
	applyFloat("CHART_CANDLE_WIDTH", &cfg.CandleWidth)

	applyAlpha("CHART_OB_FILL_R", "CHART_OB_FILL_G", "CHART_OB_FILL_B", "CHART_OB_FILL_A", &cfg.OBFill)
	applyColor("CHART_OB_BORDER", &cfg.OBBorder)
	applyString("CHART_OB_LABEL", &cfg.OBLabel)

	applyAlpha("CHART_FVG_FILL_R", "CHART_FVG_FILL_G", "CHART_FVG_FILL_B", "CHART_FVG_FILL_A", &cfg.FVGFill)
	applyColor("CHART_FVG_BORDER", &cfg.FVGBorder)
	applyString("CHART_FVG_LABEL", &cfg.FVGLabel)

	applyColor("CHART_CHANNEL_COLOR", &cfg.ChannelColor)
	if v, ok := env["CHART_CHANNEL_FILL_A"]; ok {
		if n, err := strconv.ParseUint(v, 10, 8); err == nil {
			cfg.ChannelFill.A = uint8(n)
		}
	}

	applyColor("CHART_SR_COLOR", &cfg.SRColor)

	if v, ok := env["CHART_ENTRY_FILL_A"]; ok {
		if n, err := strconv.ParseUint(v, 10, 8); err == nil {
			cfg.EntryZoneFill.A = uint8(n)
		}
	}
	applyColor("CHART_ENTRY_BORDER", &cfg.EntryZoneBorder)
	applyString("CHART_ENTRY_LABEL", &cfg.EntryLabel)

	applyColor("CHART_SL_COLOR", &cfg.SLColor)
	if v, ok := env["CHART_SL_FILL_A"]; ok {
		if n, err := strconv.ParseUint(v, 10, 8); err == nil {
			cfg.SLFill.A = uint8(n)
		}
	}
	applyString("CHART_SL_LABEL", &cfg.SLLabel)

	applyColor("CHART_TP_COLOR", &cfg.TPColor)
	if v, ok := env["CHART_TP_FILL_A"]; ok {
		if n, err := strconv.ParseUint(v, 10, 8); err == nil {
			cfg.TPFill.A = uint8(n)
		}
	}
	applyString("CHART_TP_LABEL", &cfg.TPLabel)

	applyColor("CHART_ARROW_COLOR", &cfg.ArrowColor)
	applyColor("CHART_CURRENT_PRICE_COLOR", &cfg.CurrentPriceColor)
	applyString("CHART_WATERMARK", &cfg.WatermarkText)
	applyColor("CHART_WATERMARK_COLOR", &cfg.WatermarkColor)
	applyString("CHART_FONT_PATH", &cfg.FontPath)
	applyString("CHART_SAVE_DIR", &cfg.SaveDir)

	return cfg
}

// readEnvFile читает key=value пары из файла, игнорируя комментарии.
func readEnvFile(path string) map[string]string {
	result := make(map[string]string)
	f, err := os.Open(path)
	if err != nil {
		return result
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Убираем inline-комментарии вида KEY=VALUE # comment
		if idx := strings.Index(line, " #"); idx > 0 {
			line = strings.TrimSpace(line[:idx])
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return result
}

// ---- цветовые утилиты ----

func hex(h string) color.RGBA {
	h = strings.TrimPrefix(h, "#")
	if len(h) != 6 {
		return color.RGBA{255, 255, 255, 255}
	}
	r, _ := strconv.ParseUint(h[0:2], 16, 8)
	g, _ := strconv.ParseUint(h[2:4], 16, 8)
	b, _ := strconv.ParseUint(h[4:6], 16, 8)
	return color.RGBA{uint8(r), uint8(g), uint8(b), 255}
}

func rgba(r, g, b, a uint8) color.RGBA {
	return color.RGBA{r, g, b, a}
}
