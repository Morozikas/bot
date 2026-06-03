package main

import (
	"fmt"
	"bybit-elite-signal/charts"
)

func main() {
	_ = charts.LoadConfig()

	// Шорт-сетап: OB и FVG выше точки входа, SL выше, TP ниже
	data := &charts.SignalChartData{
		Symbol:        "BTCUSDT",
		Direction:     "Short",
		Timeframe:     "15m",
		Price:         65463.9,
		EntryZoneLow:  65800.0,
		EntryZoneHigh: 66100.0,
		EntryIdeal:    65955.1,
		StopLoss:      66418.6,  // выше Entry (шорт)
		TakeProfit:    65137.7,  // ниже Entry (шорт)
		RR:            2.4,
		// Медвежий OB — выше Entry (где продавцы)
		OBLow:  65955.1,
		OBHigh: 66418.6,
		// FVG — ещё выше (незакрытый имбаланс)
		FVGLow:  66200.0,
		FVGHigh: 66700.0,
		Score:   91,
		Catalyst: "Bearish OB + FVG + CHOCH",
		Mode:    "⚡ INTRADAY",
		Deadline: "~4ч",
	}

	png, err := charts.GenerateFromRaw(data)
	if err != nil {
		fmt.Println("ERROR:", err)
		return
	}
	fmt.Printf("OK — %d байт\n", len(png))
}
