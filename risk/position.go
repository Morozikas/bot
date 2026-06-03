// Пакет risk — управление рисками.
// position.go — расчёт размера позиции по % риска от капитала.
package risk

import (
	"fmt"
	"math"

	"bybit-elite-signal/config"
)

// PositionParams — рассчитанные параметры позиции.
type PositionParams struct {
	Symbol     string
	Direction  string
	Entry      float64
	StopLoss   float64
	TakeProfit float64
	Leverage   int
	Quantity   float64
	Notional   float64
	RiskUSDT   float64
	MarginReq  float64
}

// Manager управляет риск-параметрами.
type Manager struct {
	trading *config.TradingConfig
	signal  *config.SignalConfig
}

// NewManager создаёт Manager.
func NewManager(t *config.TradingConfig, s *config.SignalConfig) *Manager {
	return &Manager{trading: t, signal: s}
}

// CalcPosition вычисляет параметры позиции.
func (m *Manager) CalcPosition(symbol, direction string, entry, sl, tp, equity, riskPct float64, leverage int, minQty, qtyStep float64) (*PositionParams, error) {
	if entry <= 0 || sl <= 0 || tp <= 0 {
		return nil, fmt.Errorf("недопустимые уровни цен")
	}
	if equity < m.trading.MinAccountBalance {
		return nil, fmt.Errorf("баланс %.2f ниже минимума", equity)
	}
	if leverage > m.trading.MaxLeverage {
		leverage = m.trading.MaxLeverage
	}
	slDist := math.Abs(entry - sl)
	if slDist == 0 {
		return nil, fmt.Errorf("расстояние до стопа = 0")
	}
	qty := equity * riskPct / slDist
	if qtyStep > 0 {
		qty = math.Floor(qty/qtyStep) * qtyStep
	}
	if qty < minQty {
		return nil, fmt.Errorf("объём %.4f < минимального %.4f", qty, minQty)
	}
	notional := qty * entry
	return &PositionParams{
		Symbol: symbol, Direction: direction,
		Entry: entry, StopLoss: sl, TakeProfit: tp,
		Leverage: leverage, Quantity: qty,
		Notional: notional, RiskUSDT: equity * riskPct,
		MarginReq: notional / float64(leverage),
	}, nil
}

// ValidateDrawdown проверяет дневной лимит потерь.
func (m *Manager) ValidateDrawdown(currentLoss, initialBalance, newRisk float64) error {
	maxLoss := initialBalance * m.trading.MaxDailyDrawdownPct / 100
	if currentLoss+newRisk > maxLoss {
		return fmt.Errorf("дневной лимит %.2f USDT достигнут", maxLoss)
	}
	return nil
}

// ValidatePositionLimits проверяет лимиты открытых позиций.
func (m *Manager) ValidatePositionLimits(totalOpen, userOpen int) error {
	if userOpen >= m.trading.MaxPositionsPerUser {
		return fmt.Errorf("лимит позиций пользователя (%d) достигнут", m.trading.MaxPositionsPerUser)
	}
	if totalOpen >= m.trading.MaxTotalPositions {
		return fmt.Errorf("глобальный лимит позиций (%d) достигнут", m.trading.MaxTotalPositions)
	}
	return nil
}
