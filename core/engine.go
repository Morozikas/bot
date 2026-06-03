// Пакет core — центральный оркестратор всей системы.
// engine.go — двухфазное сканирование с воркер-пулом для параллельных REST-запросов.
//
// Принцип работы воркеров:
//   Все воркеры разделяют ОДИН rate limiter в api/client.go.
//   Лимит Bybit (80 req/min) не превышается даже с N воркеров.
//   Выигрыш: пока Worker 1 ждёт сетевой ответ (~300ms),
//   Worker 2 уже отправляет следующий запрос (получив токен через 750ms).
//   Это скрывает сетевую задержку и ускоряет фазу 2 в ~2–3 раза.
//
//  Нагрузка (устойчивое состояние, TTL-кеш работает):
//    Фаза 1: 1 вызов GetTickers
//    Фаза 2: ~8–12 вызовов (только устаревшие данные)
//    Итого:  ~10–13 вызовов / скан  ≈ 0.05–0.06 req/sec при 200s интервале
package core

import (
	"fmt"
	"sync"
	"time"

	"bybit-elite-signal/api"
	"bybit-elite-signal/config"
	"bybit-elite-signal/market_data/candles"
	"bybit-elite-signal/market_data/funding"
	"bybit-elite-signal/market_data/liquidations"
	mdob "bybit-elite-signal/market_data/orderbook"
	"bybit-elite-signal/market_data/oi"
	"bybit-elite-signal/market_data/trades"
	"bybit-elite-signal/scanners"
	signalengine "bybit-elite-signal/signal_engine"

	"go.uber.org/zap"
)

// fetchJob — одна единица работы для воркера.
type fetchJob struct {
	symbol  string
	jobType string // "candles_W","candles_D","candles_240","candles_60","candles_15","candles_5","candles_1"
	// "oi","funding","liquidations","orderbook","trades"
}

// Engine — центральный оркестратор.
type Engine struct {
	cfg     *config.AppConfig
	log     *zap.Logger
	client  *api.Client

	candles      *candles.Manager
	orderbook    *mdob.Manager
	tradesFeed   *trades.Feed
	fundingMgr   *funding.Manager
	oiMgr        *oi.Manager
	liquidations *liquidations.Manager

	scanner      *scanners.MarketScanner
	builder      *signalengine.Builder
	pendingZones *signalengine.PendingZoneManager

	// tickerCache — тикеры из последнего GetTickers(""), больше не перезапрашиваем
	tickerCache map[string]*api.Ticker
}

// NewEngine создаёт и инициализирует Engine.
func NewEngine(cfg *config.AppConfig, log *zap.Logger) (*Engine, error) {
	client, err := api.NewClient(
		cfg.Bybit.BaseURL, cfg.Bybit.APIKey, cfg.Bybit.APISecret, cfg.Network.ProxyURL,
	)
	if err != nil {
		return nil, fmt.Errorf("API клиент: %w", err)
	}

	e := &Engine{
		cfg:          cfg,
		log:          log,
		client:       client,
		candles:      candles.NewManager(client, log, 300),
		orderbook:    mdob.NewManager(client, log),
		tradesFeed:   trades.NewFeed(client, log, 500),
		fundingMgr:   funding.NewManager(client, log),
		oiMgr:        oi.NewManager(client, log),
		liquidations: liquidations.NewManager(client, log),
		scanner:      scanners.NewMarketScanner(client, &cfg.Signal, log),
		builder:      signalengine.NewBuilder(cfg),
		pendingZones: signalengine.NewPendingZoneManager(),
		tickerCache:  make(map[string]*api.Ticker),
	}
	// Предзагрузка BTC и ETH нужна для корреляций
	go e.candles.Fetch("BTCUSDT")
	go e.candles.Fetch("ETHUSDT")
	return e, nil
}

// SetBanNotify регистрирует коллбэк для уведомления о IP-бане Bybit.
func (e *Engine) SetBanNotify(fn func(string)) {
	e.client.SetBanCallback(func(d time.Duration) {
		if fn != nil {
			fn(fmt.Sprintf(
				"🚫 <b>Bybit заблокировал IP</b>\n\n"+
					"Причина: превышен лимит запросов (10006)\n"+
					"Пауза: <b>%s</b>\n\n"+
					"<b>Немедленные действия:</b>\n"+
					"1. Установите <code>PROXY_URL=socks5://user:pass@ip:port</code> в .env\n"+
					"2. Увеличьте <code>SCAN_INTERVAL_SEC</code> (рекомендуется 300+)\n"+
					"3. Уменьшите <code>ANALYSIS_LIMIT</code> до 5-10\n\n"+
					"Бот возобновит работу автоматически через %s",
				d.Round(time.Second), d.Round(time.Second),
			))
		}
		e.log.Error("BYBIT IP БАН",
			zap.Duration("pause", d),
			zap.String("action", "установите PROXY_URL в .env"),
		)
	})
}

// Scan выполняет один цикл и возвращает сигналы.
func (e *Engine) Scan() []*signalengine.Signal {
	sigs, _ := e.ScanWithTop()
	return sigs
}

// ScanWithTop выполняет двухфазное сканирование с воркер-пулом.
func (e *Engine) ScanWithTop() ([]*signalengine.Signal, []scanners.SymbolScore) {
	scanTotal := time.Now()

	// ══════════════════════════════════════════════════════════════════
	// ФАЗА 1 — 1 вызов: ранжирование + кеш тикеров
	// ══════════════════════════════════════════════════════════════════
	t1 := time.Now()
	scanResult := e.scanner.Scan()
	if scanResult == nil || len(scanResult.Scores) == 0 {
		return nil, nil
	}
	e.tickerCache = scanResult.Tickers
	phase1 := time.Since(t1)

	allSymbols := scanners.Symbols(scanResult.Scores)

	analysisLimit := e.cfg.Signal.AnalysisLimit
	if analysisLimit <= 0 || analysisLimit >= len(allSymbols) {
		analysisLimit = len(allSymbols)
	}
	deepSymbols := allSymbols[:analysisLimit]

	e.log.Info("⏱ Фаза 1 — ранжирование",
		zap.Duration("time", phase1),
		zap.Int("ranked", len(allSymbols)),
		zap.Int("deep", len(deepSymbols)),
	)

	// ══════════════════════════════════════════════════════════════════
	// ФАЗА 2 — воркер-пул: загрузка данных (rate-limited)
	// ══════════════════════════════════════════════════════════════════
	t2 := time.Now()
	totalCalls := e.fetchAllParallel(deepSymbols)
	phase2 := time.Since(t2)

	e.log.Info("⏱ Фаза 2 — загрузка данных",
		zap.Duration("time", phase2),
		zap.Int("rest_calls", totalCalls),
		zap.Int("workers", e.cfg.Signal.WorkerCount),
	)

	// ══════════════════════════════════════════════════════════════════
	// ФАЗА 3 — CPU-анализ: без сетевых вызовов
	// ══════════════════════════════════════════════════════════════════
	t3 := time.Now()
	results := make([]*signalengine.Signal, 0)
	for _, sym := range deepSymbols {
		if sig := e.analyzeSymbol(sym); sig != nil {
			results = append(results, sig)
		}
	}
	phase3 := time.Since(t3)

	e.log.Info("⏱ Фаза 3 — анализ",
		zap.Duration("time", phase3),
		zap.Int("signals", len(results)),
		zap.Duration("total", time.Since(scanTotal)),
	)
	return results, scanResult.Scores
}

// fetchAllParallel загружает данные для всех символов через воркер-пул.
// Воркеры разделяют rate limiter клиента — лимит Bybit не превышается.
func (e *Engine) fetchAllParallel(symbols []string) int {
	numWorkers := e.cfg.Signal.WorkerCount
	if numWorkers < 1 {
		numWorkers = 1
	}
	if numWorkers > 10 {
		numWorkers = 10 // жёсткий потолок безопасности
	}

	// Собираем только реально нужные задачи (TTL-кеш фильтрует ненужные)
	jobs := e.buildJobQueue(symbols)
	if len(jobs) == 0 {
		e.log.Debug("фаза 2: все данные актуальны, REST запросы не нужны")
		return 0
	}

	e.log.Debug("воркер-пул запущен",
		zap.Int("workers", numWorkers),
		zap.Int("jobs", len(jobs)),
	)

	jobCh := make(chan fetchJob, len(jobs))
	for _, j := range jobs {
		jobCh <- j
	}
	close(jobCh)

	var (
		wg         sync.WaitGroup
		mu         sync.Mutex
		totalCalls int
	)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for job := range jobCh {
				n := e.executeJob(job)
				if n > 0 {
					mu.Lock()
					totalCalls += n
					mu.Unlock()
				}
			}
		}(i)
	}
	wg.Wait()
	return totalCalls
}

// buildJobQueue формирует список задач, фильтруя уже актуальные данные.
func (e *Engine) buildJobQueue(symbols []string) []fetchJob {
	jobs := make([]fetchJob, 0, len(symbols)*7)
	tfs := []string{"D", "240", "60", "15", "5", "1"}

	for _, sym := range symbols {
		// Свечи: добавляем задачу только для устаревших таймфреймов
		for _, tf := range tfs {
			if !e.candles.IsFresh(sym, tf) {
				jobs = append(jobs, fetchJob{sym, "candles_" + tf})
			}
		}
		// Деривативы: добавляем только если TTL истёк
		if e.oiMgr.IsStale(sym) {
			jobs = append(jobs, fetchJob{sym, "oi"})
		}
		if e.fundingMgr.IsStale(sym) {
			jobs = append(jobs, fetchJob{sym, "funding"})
		}
		if e.liquidations.IsStale(sym) {
			jobs = append(jobs, fetchJob{sym, "liquidations"})
		}
		// Стакан и сделки не загружаем — Footprint и Heatmap убраны из pipeline
	}
	return jobs
}

// executeJob выполняет одну задачу и возвращает 1 если был REST вызов.
func (e *Engine) executeJob(job fetchJob) int {
	sym := job.symbol
	switch {
	case len(job.jobType) > 8 && job.jobType[:8] == "candles_":
		tf := job.jobType[8:]
		// Точечная загрузка одного таймфрейма
		c, err := e.client.GetCandles(sym, tf, 300)
		if err != nil {
			e.log.Warn("ошибка свечей", zap.String("symbol", sym), zap.String("tf", tf), zap.Error(err))
			return 0
		}
		e.candles.SetTF(sym, tf, c)
		return 1

	case job.jobType == "oi":
		if e.oiMgr.Fetch(sym, 200) {
			return 1
		}
	case job.jobType == "funding":
		if e.fundingMgr.Fetch(sym, 50) {
			return 1
		}
	case job.jobType == "liquidations":
		if e.liquidations.Fetch(sym) {
			return 1
		}
	case job.jobType == "orderbook":
		e.orderbook.Fetch(sym, 200)
		return 1
	case job.jobType == "trades":
		e.tradesFeed.Fetch(sym, 200)
		return 1
	}
	return 0
}

// analyzeSymbol — только CPU, без сетевых вызовов.
func (e *Engine) analyzeSymbol(symbol string) *signalengine.Signal {
	ticker := e.tickerCache[symbol]
	if ticker == nil {
		return nil
	}
	markPrice, _ := ticker.MarkPrice.Float64()
	return e.builder.Build(signalengine.BuildInput{
		Symbol:       symbol,
		MarkPrice:    markPrice,
		Candles1H:    e.candles.Get(symbol, "60"),
		Candles15M:   e.candles.Get(symbol, "15"),
		Candles5M:    e.candles.Get(symbol, "5"),
		BTCCandles:   e.candles.Get("BTCUSDT", "60"),
		ETHCandles:   e.candles.Get("ETHUSDT", "60"),
		OIHistory:    e.oiMgr.Get(symbol),
		FundingHist:  e.fundingMgr.Get(symbol),
		Liquidations: e.liquidations.Get(symbol),
	})
}

// ScanBestSetup возвращает лучший сетап без пороговых фильтров (для /scan).
func (e *Engine) ScanBestSetup(deepSymbols []string) *signalengine.Signal {
	var best *signalengine.Signal
	for _, sym := range deepSymbols {
		ticker := e.tickerCache[sym]
		if ticker == nil {
			continue
		}
		markPrice, _ := ticker.MarkPrice.Float64()
		sig := e.builder.BuildBestEffort(signalengine.BuildInput{
			Symbol: sym, MarkPrice: markPrice,
			Candles1H:  e.candles.Get(sym, "60"),
			Candles15M: e.candles.Get(sym, "15"),
			Candles5M:  e.candles.Get(sym, "5"),
			BTCCandles: e.candles.Get("BTCUSDT", "60"),
			ETHCandles: e.candles.Get("ETHUSDT", "60"),
			OIHistory:  e.oiMgr.Get(sym),
			FundingHist: e.fundingMgr.Get(sym),
			Liquidations: e.liquidations.Get(sym),
		})
		if sig == nil {
			continue
		}
		if best == nil || sig.Score > best.Score {
			best = sig
		}
	}
	return best
}

// PendingZones возвращает менеджер зон ожидания.
func (e *Engine) PendingZones() *signalengine.PendingZoneManager {
	return e.pendingZones
}

// CurrentPrices возвращает текущие mark price для всех символов в кеше тикеров.
func (e *Engine) CurrentPrices() map[string]float64 {
	prices := make(map[string]float64, len(e.tickerCache))
	for sym, t := range e.tickerCache {
		if t != nil {
			p, _ := t.MarkPrice.Float64()
			prices[sym] = p
		}
	}
	return prices
}

// CheckPendingZones проверяет зоны ожидания.
// Когда цена входит в зону — дополнительно проверяет подтверждающую свечу.
// Сигнал срабатывает ТОЛЬКО при наличии реакции рынка на уровне.
func (e *Engine) CheckPendingZones() []*signalengine.PendingZone {
	prices := e.CurrentPrices()
	if len(prices) == 0 {
		return nil
	}
	tol := e.cfg.Signal.PendingEntryTolerancePct

	// Получаем список кандидатов (цена вошла в зону)
	candidates := e.pendingZones.CheckAndFire(prices, tol)
	if len(candidates) == 0 {
		return nil
	}

	// Фильтруем: только те где есть подтверждение на 5M или 1M
	confirmed := make([]*signalengine.PendingZone, 0, len(candidates))
	for _, zone := range candidates {
		if e.hasConfirmationCandle(zone) {
			confirmed = append(confirmed, zone)
			e.log.Info("pending zone подтверждена свечой",
				zap.String("symbol", zone.Symbol),
				zap.String("direction", zone.Direction),
			)
		} else {
			e.log.Debug("pending zone без подтверждения — пропускаем",
				zap.String("symbol", zone.Symbol),
				zap.String("direction", zone.Direction),
			)
			// Возвращаем зону обратно в список — ждём подтверждения
			e.pendingZones.Add(zone)
		}
	}
	return confirmed
}

// hasConfirmationCandle проверяет наличие подтверждающей свечи при касании зоны.
//
// Для LONG подтверждение:
//   - Последняя 5M свеча: close > open (бычья реакция)
//   - Нижняя тень свечи коснулась зоны (low <= zone.ZoneTop + buffer)
//   - Close выше ZoneBot (не пробили зону насквозь)
//   - Объём выше среднего (рынок отреагировал)
//
// Для SHORT — зеркально.
func (e *Engine) hasConfirmationCandle(zone *signalengine.PendingZone) bool {
	// Пробуем 5M, потом 1M
	for _, tf := range []string{"5", "1"} {
		candles := e.candles.Get(zone.Symbol, tf)
		if len(candles) < 5 {
			continue
		}

		last := candles[len(candles)-1]
		prev := candles[len(candles)-2]

		o, _ := last.Open.Float64()
		c, _ := last.Close.Float64()
		h, _ := last.High.Float64()
		l, _ := last.Low.Float64()
		v, _ := last.Volume.Float64()

		// Средний объём за 10 свечей
		avgVol := 0.0
		for _, cv := range candles[len(candles)-11 : len(candles)-1] {
			vv, _ := cv.Volume.Float64()
			avgVol += vv
		}
		avgVol /= 10

		prevO, _ := prev.Open.Float64()
		prevC, _ := prev.Close.Float64()
		_ = prevO
		_ = prevC

		buffer := zone.ATR14 * 0.2

		switch zone.Direction {
		case "Long":
			bullishCandle := c > o                               // бычья свеча
			touchedZone := l <= zone.ZoneTop+buffer              // нижняя тень коснулась зоны
			notBroken := c >= zone.ZoneBot                       // закрытие выше нижней границы
			volumeOK := v >= avgVol*0.8                          // объём не ниже среднего

			// Дополнительно: пин-бар или поглощение
			body := c - o
			totalRange := h - l
			pinBar := totalRange > 0 && body/totalRange < 0.5 && l < zone.ZoneTop // длинная нижняя тень

			if touchedZone && notBroken && volumeOK && (bullishCandle || pinBar) {
				return true
			}

		case "Short":
			bearishCandle := c < o
			touchedZone := h >= zone.ZoneBot-buffer
			notBroken := c <= zone.ZoneTop
			volumeOK := v >= avgVol*0.8

			body := o - c
			totalRange := h - l
			pinBar := totalRange > 0 && body/totalRange < 0.5 && h > zone.ZoneBot

			if touchedZone && notBroken && volumeOK && (bearishCandle || pinBar) {
				return true
			}
		}
	}
	return false
}

// RegisterPendingZone добавляет найденный OB-сетап как зону ожидания.
// Вызывается из analyzeSymbol если сигнал найден, но цена ещё не в зоне.
func (e *Engine) RegisterPendingZone(sig *signalengine.Signal, mode signalengine.TradeMode, tps []float64, atr1h float64) {
	if sig == nil {
		return
	}
	maxAge := time.Duration(e.cfg.Signal.PendingZoneMaxAgeSec) * time.Second
	tpForDeadline := sig.TakeProfit
	if len(tps) > 0 {
		tpForDeadline = tps[len(tps)-1]
	}
	zone := &signalengine.PendingZone{
		ID:        sig.ID,
		Symbol:    sig.Symbol,
		Direction: sig.Direction,
		Mode:      mode,
		ZoneTop:   sig.Entry * 1.001, // небольшой буфер вокруг entry
		ZoneBot:   sig.Entry * 0.999,
		SL:        sig.StopLoss,
		TPs:       tps,
		Score:     sig.Score,
		ATR14:     atr1h,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(maxAge),
		Deadline:  signalengine.EstimateDeadline(mode, sig.Entry, tpForDeadline, atr1h),
	}
	e.pendingZones.Add(zone)
	e.log.Debug("зона ожидания добавлена",
		zap.String("symbol", sig.Symbol),
		zap.String("direction", sig.Direction),
		zap.String("mode", string(mode)),
		zap.String("deadline", zone.Deadline),
		zap.Int("pending_total", e.pendingZones.Count()),
	)
}
