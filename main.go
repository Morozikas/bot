// main.go — точка входа BYBIT FUTURES ELITE SIGNAL ENGINE.
package main

import (
	"fmt"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"bybit-elite-signal/charts"
	"bybit-elite-signal/config"
	"bybit-elite-signal/core"
	signalengine "bybit-elite-signal/signal_engine"
	"bybit-elite-signal/statistics"
	"bybit-elite-signal/telegram"
	"bybit-elite-signal/trading"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic("конфиг: " + err.Error())
	}

	log := newLogger(cfg.Log.Level)
	defer log.Sync()
	log.Info("BYBIT FUTURES ELITE SIGNAL ENGINE запускается",
		zap.String("trade_mode", cfg.Signal.TradeMode),
		zap.Int("tp_count", cfg.Signal.TPCount),
	)

	container, err := core.Build(cfg, log)
	if err != nil {
		log.Fatal("ошибка инициализации", zap.Error(err))
	}

	container.Monitor.Start()

	// Подключаем уведомление о IP-бане
	container.Engine.SetBanNotify(func(msg string) {
		if container.TgBot != nil {
			container.TgBot.NotifyError(msg)
		}
		log.Error("IP БАН BYBIT — см. Telegram для инструкций")
	})

	done := make(chan struct{})
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// ── Монитор позиций (TP/SL/BE) ───────────────────────────────────────────
	if container.PositionMonitor != nil {
		go container.PositionMonitor.Run(done)
		log.Info("монитор позиций запущен (TP/SL/BE)")
	}

	// ── Проверка pending zones (отдельный тикер) ─────────────────────────────
	pendingTicker := time.NewTicker(time.Duration(cfg.Signal.PendingZoneCheckSec) * time.Second)
	defer pendingTicker.Stop()
	go func() {
		for {
			select {
			case <-done:
				return
			case <-pendingTicker.C:
				fired := container.Engine.CheckPendingZones()
				for _, zone := range fired {
					publishZoneSignal(zone, container, cfg, log)
				}
			}
		}
	}()

	// ── Коллбэк: автоматические сигналы ─────────────────────────────────────
	container.Scheduler.OnSignal(func(sigs []*signalengine.Signal) {
		for _, sig := range sigs {
			container.Monitor.RecordSignal()

			container.Stats.Add(statistics.Record{
				ID: sig.ID, Symbol: sig.Symbol, Direction: sig.Direction,
				Entry: sig.Entry, SL: sig.StopLoss, TP: sig.TakeProfit,
				RR: sig.RR, Score: sig.Score, CreatedAt: sig.CreatedAt,
				Outcome: "Pending",
			})

			log.Info("СИГНАЛ",
				zap.String("symbol", sig.Symbol),
				zap.String("direction", sig.Direction),
				zap.Float64("score", sig.Score),
				zap.Float64("rr", sig.RR),
			)

			tps, mode := calcTPs(sig, cfg)
			deadline := signalengine.EstimateDeadline(mode, sig.Entry, tps[len(tps)-1], sig.MarkPrice*0.01)

			// Проверяем: цена уже в зоне → отправляем сразу
			// Иначе → сохраняем как pending zone
			prices := container.Engine.CurrentPrices()
			price := prices[sig.Symbol]
			if isInEntryZone(sig, price, cfg.Signal.PendingEntryTolerancePct) {
				// Цена уже в зоне — публикуем немедленно
				sendSignal(sig, tps, mode, deadline, container, cfg, log)
			} else {
				// Регистрируем pending zone
				atr1h := sig.ATR1H()
				container.Engine.RegisterPendingZone(sig, mode, tps, atr1h)
				if container.TgBot != nil {
					container.TgBot.NotifyInfo(fmt.Sprintf(
						"🔍 <b>Зона ожидания: %s %s</b>\n"+
							"Вход: <code>%.4f</code> | Дедлайн: <b>%s</b>\n"+
							"Score: %.0f | RR: 1:%.1f",
						sig.Direction, sig.Symbol, sig.Entry, deadline,
						sig.Score, sig.RR,
					))
				}
			}
		}
	})

	container.Scheduler.OnScanDone(func() {
		container.Monitor.RecordScan()
	})

	// ── Команды Telegram ─────────────────────────────────────────────────────
	if container.TgBot != nil {
		cmdHandler := container.TgBot.NewCommandHandler(
			container.Log,
			func() string {
				base := container.Monitor.StatusString()
				if container.PositionMonitor != nil {
					base += fmt.Sprintf("\n📈 Активных позиций: %d", container.PositionMonitor.ActiveCount())
				}
				base += fmt.Sprintf("\n⏳ Pending зон: %d", container.Engine.PendingZones().Count())
				return base
			},
			func() { container.Scheduler.Pause() },
			func() { container.Scheduler.Resume() },
		)

		cmdHandler.SetScanFn(func() *telegram.ScanResult {
			// Используем ScanNow: если кеш свежий — возвращает мгновенно,
			// если устарел — запускает новый скан (но он запускается в фоне автосканером,
			// поэтому кеш почти всегда актуален).
			cached, fromCache := container.Scheduler.ScanNow()
			if cached == nil {
				return nil
			}

			coins := make([]telegram.CoinRank, 0, len(cached.TopScores))
			for _, s := range cached.TopScores {
				coins = append(coins, telegram.CoinRank{Symbol: s.Symbol, Score: s.MarketOpportunity})
			}
			result := &telegram.ScanResult{
				TopCoins:     coins,
				ScannedCount: len(cached.TopScores),
				SignalsFound: len(cached.Signals),
				FromCache:    fromCache,
				CacheAge:     time.Since(cached.ScannedAt),
			}

			// Квалифицированный сигнал
			if len(cached.Signals) > 0 {
				best := cached.Signals[0]
				for _, s := range cached.Signals[1:] {
					if s.Score > best.Score {
						best = s
					}
				}
				result.BestSignal = telegram.FormatSignal(best)
			}

			// ── Лучший сетап из кеша планировщика (уже рассчитан, бесплатно) ──
			// Если в кеше нет BestSetup — берём топ-15 и запускаем только CPU-анализ.
			// ScanBestSetup не делает сетевых вызовов, только читает из памяти.
			if cached.BestSetup != nil {
				bestSig := cached.BestSetup
				tps, _ := calcTPs(bestSig, cfg)
				deadline := signalengine.EstimateDeadline(
					func() signalengine.TradeMode {
						if cfg.Signal.TradeMode == "swing" {
							return signalengine.ModeSwing
						}
						return signalengine.ModeIntraday
					}(),
					bestSig.Entry, tps[len(tps)-1], bestSig.ATR14_1H,
				)
				modeLabel := "⚡ INTRADAY"
				if cfg.Signal.TradeMode == "swing" {
					modeLabel = "📊 SWING"
				}
				pngBytes, err := charts.GenerateForSignal(bestSig, tps, modeLabel, deadline)
				if err == nil {
					result.ChartBytes = pngBytes
					result.ChartCaption = fmt.Sprintf(
						"<b>%s %s</b>  |  Score: <b>%.0f/100</b>  |  RR: 1:%.1f  |  ⏰ %s\n\n"+
							"Entry: <code>%.4f</code>  SL: <code>%.4f</code>  TP: <code>%.4f</code>",
						bestSig.Direction, bestSig.Symbol, bestSig.Score, bestSig.RR, deadline,
						bestSig.Entry, bestSig.StopLoss, bestSig.TakeProfit,
					)
				}
				result.BestScore = bestSig.Score
				result.BestGrade = string(bestSig.Grade)
				result.BestSetup = bestSig.Setup
			} else if len(cached.TopScores) > 0 {
				// BestSetup не в кеше — считаем только CPU (топ-15, мгновенно)
				limit := 15
				if limit > len(cached.TopScores) {
					limit = len(cached.TopScores)
				}
				symbols := make([]string, limit)
				for i, s := range cached.TopScores[:limit] {
					symbols[i] = s.Symbol
				}
				bestSig := container.Scheduler.Engine().ScanBestSetup(symbols)
				if bestSig != nil {
					tps, _ := calcTPs(bestSig, cfg)
					modeLabel := "⚡ INTRADAY"
					deadline := signalengine.EstimateDeadline(signalengine.ModeIntraday,
						bestSig.Entry, tps[len(tps)-1], bestSig.ATR14_1H)
					pngBytes, err := charts.GenerateForSignal(bestSig, tps, modeLabel, deadline)
					if err == nil {
						result.ChartBytes = pngBytes
						result.ChartCaption = fmt.Sprintf(
							"<b>%s %s</b>  |  Score: <b>%.0f/100</b>  |  RR: 1:%.1f  |  ⏰ %s\n\n"+
								"Entry: <code>%.4f</code>  SL: <code>%.4f</code>  TP: <code>%.4f</code>",
							bestSig.Direction, bestSig.Symbol, bestSig.Score, bestSig.RR, deadline,
							bestSig.Entry, bestSig.StopLoss, bestSig.TakeProfit,
						)
					}
					result.BestScore = bestSig.Score
					result.BestGrade = string(bestSig.Grade)
					result.BestSetup = bestSig.Setup
				}
			}
			return result
		})

		cmdHandler.SetSettingsFn(func() string { return formatSettings(cfg) })
		go cmdHandler.StartListening()
	}

	// ── Запуск ───────────────────────────────────────────────────────────────
	go container.Scheduler.Run(done)

	<-quit
	log.Info("завершение работы")
	close(done)
}

// sendSignal форматирует сигнал с несколькими TP, дедлайном и отправляет.
func sendSignal(sig *signalengine.Signal, tps []float64, mode signalengine.TradeMode, deadline string,
	container *core.Container, cfg *config.AppConfig, log *zap.Logger) {

	modeLabel := "⚡ INTRADAY"
	if mode == signalengine.ModeSwing {
		modeLabel = "📊 SWING"
	}

	formatted := telegram.FormatSignalFull(sig, tps, modeLabel, deadline)

	if container.TgBot != nil {
		// Генерируем PNG-график и отправляем как фото
		pngBytes, chartErr := charts.GenerateForSignal(sig, tps, modeLabel, deadline)
		if chartErr == nil {
			caption := fmt.Sprintf("<b>%s %s</b>  |  Score: <b>%.0f</b>  |  RR: 1:%.1f  |  ⏰ %s",
				sig.Direction, sig.Symbol, sig.Score, sig.RR, deadline)
			container.TgBot.NotifySignalWithChart(pngBytes, caption)
		} else {
			// Fallback на текстовый сигнал если график не сгенерировался
			container.TgBot.NotifyInfo(formatted)
		}
	}

	// Регистрируем в мониторе позиций
	if container.PositionMonitor != nil {
		container.PositionMonitor.Add(sig, mode, tps)
	}

	// Автоисполнение
	if container.Executor != nil {
		results := container.Executor.Execute(sig)
		if msg := formatExec(results, sig.Symbol, sig.Direction); msg != "" && container.TgBot != nil {
			container.TgBot.NotifyExecution(msg)
		}
	}
}

// publishZoneSignal публикует сигнал когда цена вошла в pending zone.
func publishZoneSignal(zone *signalengine.PendingZone, container *core.Container, cfg *config.AppConfig, log *zap.Logger) {
	log.Info("pending zone сработала",
		zap.String("symbol", zone.Symbol),
		zap.String("direction", zone.Direction),
		zap.String("mode", string(zone.Mode)),
		zap.Duration("waited", time.Since(zone.CreatedAt)),
	)
	// Строим заглушку Signal из зоны для уведомления
	// (полные данные уже были рассчитаны при обнаружении OB)
	msg := fmt.Sprintf(
		"🎯 <b>ЦЕНА В ЗОНЕ: %s %s</b>\n"+
			"Вход сейчас: <code>%.4f</code>\n"+
			"SL: <code>%.4f</code>\n"+
			"%s\n"+
			"⏰ %s",
		zone.Direction, zone.Symbol, zone.ZoneTop, zone.SL,
		formatTPsText(zone.TPs, zone.Direction),
		zone.Deadline,
	)
	if container.TgBot != nil {
		container.TgBot.NotifyInfo(msg)
	}

	// Регистрируем в мониторе с временным Signal объектом
	if container.PositionMonitor != nil {
		fakeSig := &signalengine.Signal{
			ID:        zone.ID,
			Symbol:    zone.Symbol,
			Direction: zone.Direction,
			Entry:     zone.ZoneTop,
			StopLoss:  zone.SL,
			TakeProfit: zone.TPs[len(zone.TPs)-1],
			Score:     zone.Score,
		}
		container.PositionMonitor.Add(fakeSig, zone.Mode, zone.TPs)
	}
}

// calcTPs рассчитывает несколько TP уровней для сигнала.
func calcTPs(sig *signalengine.Signal, cfg *config.AppConfig) ([]float64, signalengine.TradeMode) {
	count := cfg.Signal.TPCount
	if count < 1 {
		count = 1
	}
	if count > 3 {
		count = 3
	}

	risk := math.Abs(sig.Entry - sig.StopLoss)
	tps := make([]float64, count)

	for i := 0; i < count; i++ {
		multiplier := float64(i+1) * sig.RR / float64(count)
		if sig.Direction == "Long" {
			tps[i] = sig.Entry + risk*multiplier
		} else {
			tps[i] = sig.Entry - risk*multiplier
		}
	}
	// Последний TP = оригинальный TP сигнала
	tps[count-1] = sig.TakeProfit

	mode := signalengine.ModeIntraday
	if cfg.Signal.TradeMode == "swing" {
		mode = signalengine.ModeSwing
	}
	return tps, mode
}

func isInEntryZone(sig *signalengine.Signal, price, tolerancePct float64) bool {
	if price <= 0 {
		return false
	}
	tol := price * tolerancePct
	switch sig.Direction {
	case "Long":
		return price >= sig.Entry-tol && price <= sig.Entry+tol
	case "Short":
		return price >= sig.Entry-tol && price <= sig.Entry+tol
	}
	return false
}

func formatTPsText(tps []float64, direction string) string {
	result := ""
	for i, tp := range tps {
		result += fmt.Sprintf("TP%d: <code>%.4f</code>", i+1, tp)
		if i < len(tps)-1 {
			result += " | "
		}
	}
	return result
}

func formatExec(results []trading.ExecutionResult, symbol, direction string) string {
	if len(results) == 0 {
		return ""
	}
	out := fmt.Sprintf("📋 <b>%s %s</b>\n", direction, symbol)
	for _, r := range results {
		if r.Success {
			out += fmt.Sprintf("✅ %s — <code>%s</code>\n", r.UserID, r.OrderID)
		} else {
			out += fmt.Sprintf("❌ %s — %s\n", r.UserID, r.Error)
		}
	}
	return out
}

func formatSettings(cfg *config.AppConfig) string {
	s := &cfg.Signal
	return fmt.Sprintf(
		"⚙️ <b>Текущие настройки</b>\n\n"+
			"<b>Режим:</b> <code>%s</code> | Тейков: <code>%d</code>\n"+
			"<b>Скан:</b> <code>%d сек</code> | Топ: <code>%d</code> → анализ: <code>%d</code>\n\n"+
			"<b>Пороги:</b>\n"+
			"  Score ≥ <code>%.0f</code> | RR ≥ <code>%.1f</code> | Prob ≥ <code>%.0f%%</code>\n\n"+
			"<b>Сопровождение:</b>\n"+
			"  BE при RR <code>%.1f</code> | Pending зоны: <code>%d сек</code>\n\n"+
			"<b>OB фильтры:</b>\n"+
			"  Freshness ≥ <code>%.1f</code> | Объём ≥ <code>%.1f×</code> | Ретестов ≤ <code>%d</code>\n\n"+
			"<i>Изменить: .env → перезапустить</i>",
		s.TradeMode, s.TPCount,
		s.ScanIntervalSec, s.TopCoinsLimit, s.AnalysisLimit,
		s.MinScorePublish, s.MinRR, s.MinProbability,
		s.BreakevenAtRR, s.PendingZoneCheckSec,
		s.OBMinFreshness, s.OBMinVolumeStrength, s.OBMaxRetestCount,
	)
}

func newLogger(level string) *zap.Logger {
	lvlMap := map[string]zapcore.Level{
		"DEBUG": zap.DebugLevel, "WARN": zap.WarnLevel, "ERROR": zap.ErrorLevel,
	}
	lvl, ok := lvlMap[level]
	if !ok {
		lvl = zap.InfoLevel
	}
	c := zap.NewProductionConfig()
	c.Level = zap.NewAtomicLevelAt(lvl)
	c.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	logger, _ := c.Build()
	return logger
}
