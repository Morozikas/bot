// Пакет core — центральный оркестратор.
// scheduler.go — планировщик сканирования с кешем последнего результата.
//
// Команда /scan работает так:
//   - Если последний скан был < ScanIntervalSec секунд назад → вернуть кеш мгновенно
//   - Если кеш устарел → запустить свежий скан (долго, но отправить "ожидайте" сразу)
package core

import (
	"sync"
	"sync/atomic"
	"time"

	"bybit-elite-signal/scanners"
	signalengine "bybit-elite-signal/signal_engine"

	"go.uber.org/zap"
)

// CachedScanResult — результат последнего сканирования с меткой времени.
type CachedScanResult struct {
	Signals   []*signalengine.Signal  // сигналы прошедшие все фильтры
	BestSetup *signalengine.Signal    // лучший сетап без порогового фильтра (для /scan)
	TopScores []scanners.SymbolScore
	ScannedAt time.Time
}

// Scheduler запускает Engine по расписанию.
type Scheduler struct {
	engine      *Engine
	intervalSec int
	paused      atomic.Bool
	log         *zap.Logger
	onSignal    func([]*signalengine.Signal)
	onScanDone  func()

	cacheMu   sync.RWMutex
	lastCache *CachedScanResult
}

// NewScheduler создаёт Scheduler.
func NewScheduler(engine *Engine, intervalSec int, log *zap.Logger) *Scheduler {
	return &Scheduler{engine: engine, intervalSec: intervalSec, log: log}
}

// Engine возвращает внутренний движок.
func (s *Scheduler) Engine() *Engine { return s.engine }

// OnSignal регистрирует коллбэк для публикации сигналов.
func (s *Scheduler) OnSignal(fn func([]*signalengine.Signal)) { s.onSignal = fn }

// OnScanDone регистрирует коллбэк после каждого сканирования.
func (s *Scheduler) OnScanDone(fn func()) { s.onScanDone = fn }

// Pause приостанавливает автоматическое сканирование.
func (s *Scheduler) Pause() { s.paused.Store(true) }

// Resume возобновляет автоматическое сканирование.
func (s *Scheduler) Resume() { s.paused.Store(false) }

// IsPaused возвращает true если автосканер на паузе.
func (s *Scheduler) IsPaused() bool { return s.paused.Load() }

// LastTop возвращает топ монет из последнего скана.
func (s *Scheduler) LastTop() []scanners.SymbolScore {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	if s.lastCache == nil {
		return nil
	}
	cp := make([]scanners.SymbolScore, len(s.lastCache.TopScores))
	copy(cp, s.lastCache.TopScores)
	return cp
}

// CachedResult возвращает последний закешированный результат скана и его возраст.
func (s *Scheduler) CachedResult() (*CachedScanResult, time.Duration) {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	if s.lastCache == nil {
		return nil, 0
	}
	return s.lastCache, time.Since(s.lastCache.ScannedAt)
}

// ScanNow выполняет немедленный скан (для команды /scan).
// Если кеш свежий (< intervalSec секунд) — возвращает его мгновенно.
// Иначе — запускает полный скан.
func (s *Scheduler) ScanNow() (*CachedScanResult, bool) {
	// Проверяем актуальность кеша
	cached, age := s.CachedResult()
	if cached != nil && age < time.Duration(s.intervalSec)*time.Second {
		s.log.Info("/scan: возврат кешированного результата", zap.Duration("age", age))
		return cached, true // true = из кеша
	}

	// Кеш устарел — полный скан
	s.log.Info("/scan: запуск свежего сканирования")
	start := time.Now()
	sigs, top := s.engine.ScanWithTop()
	elapsed := time.Since(start)
	s.log.Info("/scan завершён", zap.Duration("elapsed", elapsed), zap.Int("signals", len(sigs)))

	// Лучший сетап среди всех проанализированных символов (без фильтров)
	deepSymbols := scanners.Symbols(top)
	analysisLimit := s.engine.cfg.Signal.AnalysisLimit
	if analysisLimit > 0 && analysisLimit < len(deepSymbols) {
		deepSymbols = deepSymbols[:analysisLimit]
	}
	bestSetup := s.engine.ScanBestSetup(deepSymbols)

	result := &CachedScanResult{
		Signals:   sigs,
		BestSetup: bestSetup,
		TopScores: top,
		ScannedAt: time.Now(),
	}
	s.cacheMu.Lock()
	s.lastCache = result
	s.cacheMu.Unlock()

	return result, false
}

// Run запускает бесконечный цикл автоматического сканирования.
func (s *Scheduler) Run(done <-chan struct{}) {
	ticker := time.NewTicker(time.Duration(s.intervalSec) * time.Second)
	defer ticker.Stop()
	s.log.Info("сканер запущен", zap.Int("interval_sec", s.intervalSec))

	for {
		select {
		case <-done:
			s.log.Info("сканер остановлен")
			return
		case <-ticker.C:
			if s.paused.Load() {
				continue
			}
			start := time.Now()
			sigs, top := s.engine.ScanWithTop()
			elapsed := time.Since(start)

			deepSymbols := scanners.Symbols(top)
			analysisLimit := s.engine.cfg.Signal.AnalysisLimit
			if analysisLimit > 0 && analysisLimit < len(deepSymbols) {
				deepSymbols = deepSymbols[:analysisLimit]
			}
			bestSetup := s.engine.ScanBestSetup(deepSymbols)

			result := &CachedScanResult{
				Signals:   sigs,
				BestSetup: bestSetup,
				TopScores: top,
				ScannedAt: time.Now(),
			}
			s.cacheMu.Lock()
			s.lastCache = result
			s.cacheMu.Unlock()

			s.log.Info("автоскан завершён",
				zap.Int("signals", len(sigs)),
				zap.Int("top_coins", len(top)),
				zap.Duration("elapsed", elapsed),
			)
			if s.onScanDone != nil {
				s.onScanDone()
			}
			if len(sigs) > 0 && s.onSignal != nil {
				s.onSignal(sigs)
			}
		}
	}
}
