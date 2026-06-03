// Пакет core — центральный оркестратор.
// position_monitor.go — отслеживание активных сделок после отправки сигнала:
//   - Фиксирует время достижения каждого TP
//   - Автоматически переносит SL в BE при достижении BreakevenAtRR
//   - Отправляет уведомления в Telegram при TP / SL / BE
package core

import (
	"fmt"
	"math"
	"sync"
	"time"

	"bybit-elite-signal/config"
	signalengine "bybit-elite-signal/signal_engine"

	"go.uber.org/zap"
)

// ActivePosition — одна отслеживаемая позиция после отправки сигнала.
type ActivePosition struct {
	Signal     *signalengine.Signal
	Mode       signalengine.TradeMode
	TPs        []float64 // все уровни TP (TP1, TP2, ...)
	CurrentSL  float64   // текущий SL (может быть перенесён в BE)
	IsAtBE     bool      // SL уже на BE
	SentAt     time.Time // время отправки сигнала
	HitTPs     []bool    // какие TP уже достигнуты
	ClosedAt   time.Time
	IsClosed   bool
	CloseReason string // "TP1" | "TP2" | "SL" | "BE"
	TimeToTP1  time.Duration // время до первого TP
}

// PositionMonitor отслеживает активные позиции и управляет сопровождением.
type PositionMonitor struct {
	cfg       *config.SignalConfig
	log       *zap.Logger
	positions map[string]*ActivePosition // key = signal.ID
	mu        sync.RWMutex
	notify    func(msg string) // коллбэк для отправки в Telegram
	getPrices func() map[string]float64 // коллбэк для получения текущих цен
}

// NewPositionMonitor создаёт монитор позиций.
func NewPositionMonitor(
	cfg *config.SignalConfig,
	log *zap.Logger,
	notifyFn func(msg string),
	getPrices func() map[string]float64,
) *PositionMonitor {
	return &PositionMonitor{
		cfg:       cfg,
		log:       log,
		positions: make(map[string]*ActivePosition),
		notify:    notifyFn,
		getPrices: getPrices,
	}
}

// Add регистрирует новую активную позицию.
func (m *PositionMonitor) Add(sig *signalengine.Signal, mode signalengine.TradeMode, tps []float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pos := &ActivePosition{
		Signal:    sig,
		Mode:      mode,
		TPs:       tps,
		CurrentSL: sig.StopLoss,
		SentAt:    time.Now(),
		HitTPs:    make([]bool, len(tps)),
	}
	m.positions[sig.ID] = pos
	m.log.Info("позиция добавлена в мониторинг",
		zap.String("symbol", sig.Symbol),
		zap.String("direction", sig.Direction),
		zap.Int("tps", len(tps)),
	)
}

// Run запускает цикл мониторинга (блокирующий).
func (m *PositionMonitor) Run(done <-chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	m.log.Info("монитор позиций запущен")

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if m.getPrices == nil {
				continue
			}
			prices := m.getPrices()
			m.checkAll(prices)
		}
	}
}

// ActiveCount возвращает число активных позиций.
func (m *PositionMonitor) ActiveCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := 0
	for _, p := range m.positions {
		if !p.IsClosed {
			count++
		}
	}
	return count
}

func (m *PositionMonitor) checkAll(prices map[string]float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, pos := range m.positions {
		if pos.IsClosed {
			// Убираем закрытые позиции старше 1 часа
			if time.Since(pos.ClosedAt) > time.Hour {
				delete(m.positions, id)
			}
			continue
		}

		price, ok := prices[pos.Signal.Symbol]
		if !ok || price <= 0 {
			continue
		}

		// Проверка SL
		if m.isSLHit(pos, price) {
			pos.IsClosed = true
			pos.ClosedAt = time.Now()
			if pos.IsAtBE {
				pos.CloseReason = "BE"
			} else {
				pos.CloseReason = "SL"
			}
			m.sendSLNotification(pos, price)
			continue
		}

		// Проверка каждого TP
		for i, tp := range pos.TPs {
			if pos.HitTPs[i] {
				continue
			}
			if m.isTPHit(pos, price, tp) {
				pos.HitTPs[i] = true
				elapsed := time.Since(pos.SentAt)
				pos.CloseReason = fmt.Sprintf("TP%d", i+1)

				// Первый TP — записываем время и переносим SL в BE
				if i == 0 {
					pos.TimeToTP1 = elapsed
					if m.cfg.TradeManagement && !pos.IsAtBE {
						pos.IsAtBE = true
						pos.CurrentSL = pos.Signal.Entry
						if m.cfg.NotifyOnBE {
							m.sendBENotification(pos)
						}
					}
				}

				// Последний TP — позиция закрыта
				if i == len(pos.TPs)-1 {
					pos.IsClosed = true
					pos.ClosedAt = time.Now()
				}

				if m.cfg.NotifyOnTP {
					m.sendTPNotification(pos, i+1, tp, elapsed, price)
				}
			}
		}
	}
}

func (m *PositionMonitor) isSLHit(pos *ActivePosition, price float64) bool {
	switch pos.Signal.Direction {
	case "Long":
		return price <= pos.CurrentSL
	case "Short":
		return price >= pos.CurrentSL
	}
	return false
}

func (m *PositionMonitor) isTPHit(pos *ActivePosition, price, tp float64) bool {
	switch pos.Signal.Direction {
	case "Long":
		return price >= tp
	case "Short":
		return price <= tp
	}
	return false
}

func (m *PositionMonitor) sendTPNotification(pos *ActivePosition, tpNum int, tp float64, elapsed time.Duration, price float64) {
	pct := math.Abs(price-pos.Signal.Entry) / pos.Signal.Entry * 100
	modeLabel := "📊 Swing"
	if pos.Mode == signalengine.ModeIntraday {
		modeLabel = "⚡ Intraday"
	}

	remaining := ""
	allHit := true
	for _, hit := range pos.HitTPs {
		if !hit {
			allHit = false
			break
		}
	}
	if !allHit {
		remaining = " | SL → BE"
	}

	msg := fmt.Sprintf(
		"✅ <b>TP%d ДОСТИГНУТ</b> %s\n"+
			"<b>%s %s</b>\n"+
			"Цена: <code>%.4f</code> (+%.2f%%)\n"+
			"Время от входа: <b>%s</b>%s",
		tpNum, modeLabel,
		pos.Signal.Direction, pos.Signal.Symbol,
		price, pct,
		formatElapsed(elapsed),
		remaining,
	)
	m.notify(msg)
}

func (m *PositionMonitor) sendSLNotification(pos *ActivePosition, price float64) {
	if !m.cfg.NotifyOnSL {
		return
	}
	pct := math.Abs(price-pos.Signal.Entry) / pos.Signal.Entry * 100
	reason := "SL"
	icon := "❌"
	if pos.IsAtBE {
		reason = "Безубыток (BE)"
		icon = "⚪"
		pct = 0
	}

	msg := fmt.Sprintf(
		"%s <b>%s сработал</b>\n"+
			"<b>%s %s</b>\n"+
			"Цена: <code>%.4f</code> (%.2f%%)\n"+
			"Время в позиции: <b>%s</b>",
		icon, reason,
		pos.Signal.Direction, pos.Signal.Symbol,
		price, pct,
		formatElapsed(time.Since(pos.SentAt)),
	)
	m.notify(msg)
}

func (m *PositionMonitor) sendBENotification(pos *ActivePosition) {
	msg := fmt.Sprintf(
		"📌 <b>SL перенесён в безубыток</b>\n"+
			"<b>%s %s</b>\n"+
			"Новый SL: <code>%.4f</code> (точка входа)",
		pos.Signal.Direction, pos.Signal.Symbol,
		pos.Signal.Entry,
	)
	m.notify(msg)
}

func formatElapsed(d time.Duration) string {
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%dс", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dм %dс", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dч %dм", int(d.Hours()), int(d.Minutes())%60)
}
