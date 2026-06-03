// Пакет core — центральный оркестратор.
// container.go — контейнер зависимостей: создаёт и связывает все компоненты.
package core

import (
	"bybit-elite-signal/config"
	"bybit-elite-signal/monitoring"
	"bybit-elite-signal/risk"
	"bybit-elite-signal/statistics"
	"bybit-elite-signal/storage"
	"bybit-elite-signal/telegram"
	"bybit-elite-signal/trading"

	"go.uber.org/zap"
)

// Container хранит все компоненты приложения.
type Container struct {
	Cfg             *config.AppConfig
	Log             *zap.Logger
	Engine          *Engine
	Scheduler       *Scheduler
	EventBus        *EventBus
	TgBot           *telegram.Bot
	Monitor         *monitoring.Monitor
	RiskMgr         *risk.Manager
	Stats           *statistics.History
	Executor        *trading.Executor
	UserMgr         *trading.UserManager
	PositionMonitor *PositionMonitor
	DB              *storage.DB
	Cache           *storage.Cache
}

// Build собирает все зависимости.
func Build(cfg *config.AppConfig, log *zap.Logger) (*Container, error) {
	c := &Container{Cfg: cfg, Log: log}
	c.EventBus = NewEventBus()
	c.Stats = statistics.NewHistory(500)
	c.Monitor = monitoring.NewMonitor(cfg.Monitoring.Port, log)
	c.RiskMgr = risk.NewManager(&cfg.Trading, &cfg.Signal)

	var err error
	c.Engine, err = NewEngine(cfg, log)
	if err != nil {
		return nil, err
	}
	c.Scheduler = NewScheduler(c.Engine, cfg.Signal.ScanIntervalSec, log)

	// Telegram
	if cfg.Telegram.BotToken != "" && cfg.Telegram.ChatID != "" {
		sender, err := telegram.NewSender(cfg.Telegram.BotToken, cfg.Telegram.ChatID, log)
		if err != nil {
			log.Warn("ошибка Telegram", zap.Error(err))
		} else {
			c.TgBot = telegram.NewBot(sender, cfg.Telegram.ExtraChatIDs, log)
		}
	}

	// Монитор позиций (TP/SL/BE) — создаётся всегда, уведомляет через TgBot
	if cfg.Signal.TradeManagement {
		var notifyFn func(string)
		if c.TgBot != nil {
			notifyFn = func(msg string) { c.TgBot.NotifyInfo(msg) }
		}
		c.PositionMonitor = NewPositionMonitor(
			&cfg.Signal,
			log,
			notifyFn,
			func() map[string]float64 { return c.Engine.CurrentPrices() },
		)
	}

	// Автоторговля
	if cfg.Trading.Enabled {
		users := config.LoadUsers()
		um, err := trading.NewUserManager(cfg, users)
		if err != nil {
			log.Warn("ошибка менеджера пользователей", zap.Error(err))
		} else {
			c.UserMgr = um
			c.Executor = trading.NewExecutor(um, c.RiskMgr, c.Engine.client, cfg, log)
			um.RefreshBalances()
		}
	}

	// БД и Redis
	if cfg.Database.URL != "" {
		db, err := storage.NewDB(cfg.Database.URL)
		if err != nil {
			log.Warn("ошибка БД", zap.Error(err))
		} else {
			c.DB = db
			_ = db.Migrate()
		}
	}
	if cfg.Redis.URL != "" {
		cache, err := storage.NewCache(cfg.Redis.URL)
		if err != nil {
			log.Warn("ошибка Redis", zap.Error(err))
		} else {
			c.Cache = cache
		}
	}

	return c, nil
}
