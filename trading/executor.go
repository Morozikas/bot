// Пакет trading — автоматическое исполнение сделок.
// executor.go — размещение лимитных ордеров на Bybit для всех пользователей параллельно.
package trading

import (
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	"bybit-elite-signal/api"
	"bybit-elite-signal/config"
	"bybit-elite-signal/risk"
	signalengine "bybit-elite-signal/signal_engine"

	"go.uber.org/zap"
)

// ExecutionResult — результат исполнения сигнала для одного пользователя.
type ExecutionResult struct {
	UserID  string
	Symbol  string
	OrderID string
	Success bool
	Error   string
}

// Executor размещает ордера для всех управляемых пользователей.
type Executor struct {
	userMgr      *UserManager
	riskMgr      *risk.Manager
	instrCache   map[string]api.InstrumentInfo
	instrMu      sync.RWMutex
	cfg          *config.AppConfig
	log          *zap.Logger
	publicClient *api.Client
}

// NewExecutor создаёт Executor.
func NewExecutor(
	userMgr *UserManager,
	riskMgr *risk.Manager,
	publicClient *api.Client,
	cfg *config.AppConfig,
	log *zap.Logger,
) *Executor {
	e := &Executor{
		userMgr:      userMgr,
		riskMgr:      riskMgr,
		instrCache:   make(map[string]api.InstrumentInfo),
		cfg:          cfg,
		log:          log,
		publicClient: publicClient,
	}
	go e.refreshInstruments()
	return e
}

// Execute размещает сигнал для всех подходящих пользователей одновременно.
func (e *Executor) Execute(sig *signalengine.Signal) []ExecutionResult {
	if !e.cfg.Trading.Enabled {
		return nil
	}
	users := e.userMgr.GetUsers()
	results := make([]ExecutionResult, 0, len(users))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, u := range users {
		u := u
		if !u.Config.Active {
			continue
		}
		if u.Config.MinScore > 0 && sig.Score < u.Config.MinScore {
			continue
		}
		if !isAllowed(sig.Symbol, u.Config.AllowedSymbols) {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			res := e.executeForUser(u, sig)
			mu.Lock()
			results = append(results, res)
			mu.Unlock()
		}()
	}
	wg.Wait()
	return results
}

func (e *Executor) executeForUser(u *ManagedUser, sig *signalengine.Signal) ExecutionResult {
	res := ExecutionResult{UserID: u.Config.ID, Symbol: sig.Symbol}

	totalOpen := 0
	for _, usr := range e.userMgr.GetUsers() {
		totalOpen += usr.OpenPositions
	}
	if err := e.riskMgr.ValidatePositionLimits(totalOpen, u.OpenPositions); err != nil {
		res.Error = err.Error()
		return res
	}

	bal, err := u.Client.GetBalance()
	if err != nil {
		res.Error = fmt.Sprintf("баланс: %v", err)
		return res
	}
	equity, _ := bal.TotalEquity.Float64()

	riskAmt := equity * u.GetRiskPct(e.cfg.Trading.DefaultRiskPct)
	if err := e.riskMgr.ValidateDrawdown(u.DailyLoss, u.InitialBalance, riskAmt); err != nil {
		res.Error = err.Error()
		return res
	}

	instr := e.getInstrument(sig.Symbol)
	leverage := u.GetLeverage(sig.Symbol, e.cfg.Trading.DefaultLeverage)
	minQty, _ := instr.LotSizeFilter.MinOrderQty.Float64()
	qtyStep, _ := instr.LotSizeFilter.QtyStep.Float64()
	tickSize, _ := instr.PriceFilter.TickSize.Float64()

	params, err := e.riskMgr.CalcPosition(
		sig.Symbol, sig.Direction,
		sig.Entry, sig.StopLoss, sig.TakeProfit,
		equity, u.GetRiskPct(e.cfg.Trading.DefaultRiskPct),
		leverage, minQty, qtyStep,
	)
	if err != nil {
		res.Error = err.Error()
		return res
	}

	if err := u.Client.SetLeverage(sig.Symbol, leverage); err != nil {
		e.log.Warn("ошибка установки плеча", zap.String("user", u.Config.ID), zap.Error(err))
	}

	// ── Финальная проверка корректности лимитного ордера ──────────────────
	// Лонг:  entry ДОЛЖЕН быть ниже markPrice  (покупаем ниже рынка)
	// Шорт:  entry ДОЛЖЕН быть выше markPrice  (продаём выше рынка)
	// sig.MarkPrice сохранён в момент генерации сигнала
	validatedEntry := validateLimitEntry(sig.Entry, sig.Direction, sig.MarkPrice, tickSize)
	if validatedEntry != sig.Entry {
		e.log.Warn("entry скорректирован для лимитного ордера",
			zap.String("symbol", sig.Symbol),
			zap.String("direction", sig.Direction),
			zap.Float64("original", sig.Entry),
			zap.Float64("adjusted", validatedEntry),
			zap.Float64("mark_price", sig.MarkPrice),
		)
	}

	side := "Buy"
	if sig.Direction == "Short" {
		side = "Sell"
	}
	req := api.OrderRequest{
		Symbol: sig.Symbol, Side: side, OrderType: e.cfg.Trading.EntryOrderType,
		Qty: fmtQty(params.Quantity, qtyStep), Price: fmtPrice(validatedEntry, tickSize),
		StopLoss: fmtPrice(params.StopLoss, tickSize), TakeProfit: fmtPrice(params.TakeProfit, tickSize),
		TimeInForce: e.cfg.Trading.TimeInForce, PositionIdx: 0,
	}

	resp, err := u.Client.PlaceOrder(req)
	if err != nil {
		res.Error = fmt.Sprintf("place order: %v", err)
		return res
	}
	u.IncrementPositions()
	res.OrderID = resp.OrderID
	res.Success = true

	if e.cfg.Trading.EntryOrderTTLSec > 0 {
		go func() {
			time.Sleep(time.Duration(e.cfg.Trading.EntryOrderTTLSec) * time.Second)
			_ = u.Client.CancelOrder(sig.Symbol, resp.OrderID)
		}()
	}
	e.log.Info("ордер размещён",
		zap.String("user", u.Config.ID), zap.String("symbol", sig.Symbol),
		zap.String("orderID", resp.OrderID), zap.Float64("qty", params.Quantity))
	return res
}

func (e *Executor) refreshInstruments() {
	instruments, err := e.publicClient.GetInstruments()
	if err != nil {
		e.log.Error("ошибка загрузки инструментов", zap.Error(err))
		return
	}
	e.instrMu.Lock()
	defer e.instrMu.Unlock()
	for _, instr := range instruments {
		e.instrCache[instr.Symbol] = instr
	}
}

func (e *Executor) getInstrument(symbol string) api.InstrumentInfo {
	e.instrMu.RLock()
	defer e.instrMu.RUnlock()
	return e.instrCache[symbol]
}

func isAllowed(symbol string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, s := range allowed {
		if s == symbol {
			return true
		}
	}
	return false
}

func fmtQty(qty, step float64) string {
	return strconv.FormatFloat(qty, 'f', countDecimals(step), 64)
}

func fmtPrice(price, tick float64) string {
	return strconv.FormatFloat(price, 'f', countDecimals(tick), 64)
}

func countDecimals(v float64) int {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	for i, c := range s {
		if c == '.' {
			return len(s) - i - 1
		}
	}
	return 0
}

// validateLimitEntry гарантирует корректность цены лимитного ордера:
//   Long  → entry < markPrice  (покупаем ниже рынка)
//   Short → entry > markPrice  (продаём выше рынка)
// Если правило нарушено — цена сдвигается вплотную к рынку (1 тик отступ).
func validateLimitEntry(entry float64, direction string, markPrice, tickSize float64) float64 {
	if markPrice <= 0 {
		return entry
	}
	tick := tickSize
	if tick <= 0 {
		tick = markPrice * 0.0001 // 0.01% как минимальный шаг если неизвестен
	}

	switch direction {
	case "Long":
		// Лонг: entry должен быть НИЖЕ markPrice
		if entry >= markPrice {
			return roundToTick(markPrice-tick, tickSize)
		}
	case "Short":
		// Шорт: entry должен быть ВЫШЕ markPrice
		if entry <= markPrice {
			return roundToTick(markPrice+tick, tickSize)
		}
	}
	return entry
}

// roundToTick округляет цену до ближайшего тика.
func roundToTick(price, tickSize float64) float64 {
	if tickSize <= 0 {
		return price
	}
	return math.Round(price/tickSize) * tickSize
}
