// Пакет signalengine — движок генерации сигналов.
// zones.go — система Pending Zones: хранит найденные OB-зоны и
// отправляет сигнал только когда цена ФАКТИЧЕСКИ входит в зону.
//
// Принцип:
//   Обнаружен OB + все подтверждения пройдены → PendingZone сохраняется
//   Фоновый воркер проверяет каждые 15 сек: цена в зоне?
//   Да → сигнал отправляется и зона удаляется
//   Нет / зона пробита / истекло время → зона удаляется без сигнала
package signalengine

import (
	"math"
	"sync"
	"time"
)

// TradeMode — режим торговли зоны.
type TradeMode string

const (
	ModeIntraday TradeMode = "intraday"
	ModeSwing    TradeMode = "swing"
)

// PendingZone — найденная зона ожидания входа.
type PendingZone struct {
	ID        string
	Symbol    string
	Direction string
	Mode      TradeMode

	// Границы зоны (OB)
	ZoneTop float64
	ZoneBot float64

	// Предрассчитанные уровни
	SL       float64
	TPs      []float64 // TP1, TP2, ... в зависимости от TPCount
	Score    float64
	ATR14    float64

	// Тайм-менеджмент
	CreatedAt  time.Time
	ExpiresAt  time.Time
	Deadline   string // человеко-читаемый дедлайн: "~4ч" или "~3 дня"

	// Флаги
	Triggered  bool
	Invalidated bool
}

// PendingZoneManager хранит активные зоны ожидания.
type PendingZoneManager struct {
	mu    sync.RWMutex
	zones map[string]*PendingZone // key = Symbol+"_"+Direction
}

// NewPendingZoneManager создаёт менеджер зон.
func NewPendingZoneManager() *PendingZoneManager {
	return &PendingZoneManager{zones: make(map[string]*PendingZone)}
}

// Add добавляет или заменяет зону ожидания для символа.
// Новая зона с более высоким Score заменяет старую.
func (m *PendingZoneManager) Add(z *PendingZone) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := z.Symbol + "_" + z.Direction + "_" + string(z.Mode)
	if existing, ok := m.zones[key]; ok {
		if z.Score <= existing.Score {
			return // не заменяем если новая зона хуже
		}
	}
	m.zones[key] = z
}

// CheckAndFire проверяет все зоны и возвращает сработавшие.
// currentPrices: map[symbol]markPrice
// tolerancePct: допуск входа в зону (например 0.001 = 0.1%)
func (m *PendingZoneManager) CheckAndFire(currentPrices map[string]float64, tolerancePct float64) []*PendingZone {
	m.mu.Lock()
	defer m.mu.Unlock()

	fired := make([]*PendingZone, 0)
	now := time.Now()

	for key, z := range m.zones {
		if z.Triggered || z.Invalidated {
			delete(m.zones, key)
			continue
		}
		// Зона истекла
		if now.After(z.ExpiresAt) {
			delete(m.zones, key)
			continue
		}

		price, ok := currentPrices[z.Symbol]
		if !ok || price <= 0 {
			continue
		}

		// Проверяем входит ли цена в зону (с допуском)
		tol := price * tolerancePct
		inZone := price >= z.ZoneBot-tol && price <= z.ZoneTop+tol

		if inZone {
			// Для лонга дополнительно проверяем что цена не прошла насквозь вниз
			if z.Direction == "Long" && price < z.ZoneBot-tol {
				z.Invalidated = true
				delete(m.zones, key)
				continue
			}
			// Для шорта — не прошла насквозь вверх
			if z.Direction == "Short" && price > z.ZoneTop+tol {
				z.Invalidated = true
				delete(m.zones, key)
				continue
			}
			z.Triggered = true
			fired = append(fired, z)
			delete(m.zones, key)
			continue
		}

		// Зона инвалидирована (цена пробила OB)
		if z.Direction == "Long" && price < z.ZoneBot*(1-tolerancePct*3) {
			z.Invalidated = true
			delete(m.zones, key)
		}
		if z.Direction == "Short" && price > z.ZoneTop*(1+tolerancePct*3) {
			z.Invalidated = true
			delete(m.zones, key)
		}
	}
	return fired
}

// Count возвращает количество активных зон.
func (m *PendingZoneManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.zones)
}

// All возвращает копию всех активных зон (для /scan).
func (m *PendingZoneManager) All() []*PendingZone {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*PendingZone, 0, len(m.zones))
	for _, z := range m.zones {
		cp := *z
		result = append(result, &cp)
	}
	return result
}

// EstimateDeadline рассчитывает примерный дедлайн на основе расстояния до TP и ATR.
func EstimateDeadline(mode TradeMode, entry, tp, atr1h float64) string {
	if atr1h <= 0 {
		if mode == ModeIntraday {
			return "~сессия"
		}
		return "~2-5 дней"
	}

	distToTP := math.Abs(tp - entry)
	// Предполагаем движение ~1 ATR в час для intraday, ~4 ATR в день для swing
	if mode == ModeIntraday {
		hours := distToTP / (atr1h * 0.8)
		switch {
		case hours < 0.5:
			return "~30 мин"
		case hours < 1:
			return "~1ч"
		case hours < 2:
			return "~1-2ч"
		case hours < 4:
			return "~2-4ч"
		default:
			return "~текущая сессия"
		}
	}
	// Swing
	dailyATR := atr1h * 6 // ~6 часов активной торговли
	if dailyATR <= 0 {
		return "~2-5 дней"
	}
	days := distToTP / dailyATR
	switch {
	case days < 1:
		return "~1 день"
	case days < 2:
		return "~1-2 дня"
	case days < 4:
		return "~2-4 дня"
	default:
		return "~неделя"
	}
}
