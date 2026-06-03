// Пакет monitoring — мониторинг работы системы: HTTP /health, /metrics, счётчики.
// monitor.go — лёгкий HTTP-сервер для проверки состояния и сбора метрик.
package monitoring

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// Monitor предоставляет HTTP-эндпоинты /health и /metrics.
type Monitor struct {
	port         int
	log          *zap.Logger
	startTime    time.Time
	signalsTotal atomic.Int64
	errorsTotal  atomic.Int64
	lastScanAt   atomic.Value
	paused       atomic.Bool
}

// NewMonitor создаёт Monitor.
func NewMonitor(port int, log *zap.Logger) *Monitor {
	m := &Monitor{port: port, log: log, startTime: time.Now()}
	m.lastScanAt.Store(time.Time{})
	return m
}

// Start запускает HTTP-сервер мониторинга.
func (m *Monitor) Start() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", m.health)
	mux.HandleFunc("/metrics", m.metrics)
	srv := &http.Server{Addr: fmt.Sprintf(":%d", m.port), Handler: mux}
	go func() {
		m.log.Info("HTTP мониторинг запущен", zap.Int("port", m.port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			m.log.Error("ошибка HTTP мониторинга", zap.Error(err))
		}
	}()
}

func (m *Monitor) health(w http.ResponseWriter, _ *http.Request) {
	status := "ok"
	if m.paused.Load() {
		status = "paused"
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": status})
}

func (m *Monitor) metrics(w http.ResponseWriter, _ *http.Request) {
	lastScan, _ := m.lastScanAt.Load().(time.Time)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"uptime_seconds": time.Since(m.startTime).Seconds(),
		"signals_total":  m.signalsTotal.Load(),
		"errors_total":   m.errorsTotal.Load(),
		"last_scan_at":   lastScan.Format(time.RFC3339),
		"paused":         m.paused.Load(),
	})
}

func (m *Monitor) RecordSignal()            { m.signalsTotal.Add(1) }
func (m *Monitor) RecordError()             { m.errorsTotal.Add(1) }
func (m *Monitor) RecordScan()              { m.lastScanAt.Store(time.Now()) }
func (m *Monitor) SetPaused(p bool)         { m.paused.Store(p) }
func (m *Monitor) IsPaused() bool           { return m.paused.Load() }

// StatusString возвращает текстовый статус для Telegram.
func (m *Monitor) StatusString() string {
	lastScan, _ := m.lastScanAt.Load().(time.Time)
	pausedStr := ""
	if m.paused.Load() {
		pausedStr = " [ПАУЗА]"
	}
	last := "никогда"
	if !lastScan.IsZero() {
		last = lastScan.Format("15:04:05")
	}
	return fmt.Sprintf(
		"⏱ Аптайм: %s%s\n📊 Сигналов: %d\n❌ Ошибок: %d\n🕐 Последнее сканирование: %s",
		time.Since(m.startTime).Round(time.Second), pausedStr,
		m.signalsTotal.Load(), m.errorsTotal.Load(), last,
	)
}
