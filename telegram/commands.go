// Пакет telegram — интеграция с Telegram.
// commands.go — поллинг входящих команд бота.
//
// ВАЖНО: команды (/scan, /status и т.д.) нужно отправлять в ЛИЧНЫЙ ЧАТ с ботом
// или в ГРУППУ где бот является участником.
// В КАНАЛЫ отправить команду невозможно — Telegram так не работает.
// Сигналы отправляются в TELEGRAM_CHAT_ID (канал/группа), а команды принимаются
// из любого чата где есть бот.
package telegram

import (
	"fmt"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

// ScanResult — результат команды /scan.
type ScanResult struct {
	// PNG-график для монеты с наивысшим Market Opportunity Score — всегда отправляется
	ChartBytes   []byte
	ChartCaption string

	// Лучший сетап с наивысшим Score (без порогового фильтра)
	BestSetup  string
	BestScore  float64
	BestGrade  string

	// Квалифицированные сигналы (прошли все пороговые фильтры)
	BestSignal   string
	SignalsFound int
	ScannedCount int
	TopCoins     []CoinRank
	FromCache    bool
	CacheAge     time.Duration
}

// CoinRank — строчка рейтинга для /top.
type CoinRank struct {
	Symbol    string
	Score     float64
	Direction string
}

// CommandHandler обрабатывает входящие команды.
type CommandHandler struct {
	bot        *tgbotapi.BotAPI
	sender     *Sender
	chatID     int64 // chat откуда пришла команда (для отправки фото)
	log        *zap.Logger
	statusFn   func() string
	pauseFn    func()
	resumeFn   func()
	scanFn     func() *ScanResult
	settingsFn func() string
}

// NewCommandHandler создаёт обработчик команд.
func NewCommandHandler(
	sender *Sender,
	log *zap.Logger,
	statusFn func() string,
	pauseFn func(),
	resumeFn func(),
) *CommandHandler {
	return &CommandHandler{
		bot:      sender.Bot(),
		sender:   sender,
		log:      log,
		statusFn: statusFn,
		pauseFn:  pauseFn,
		resumeFn: resumeFn,
	}
}

// SetScanFn регистрирует функцию для /scan.
func (c *CommandHandler) SetScanFn(fn func() *ScanResult) { c.scanFn = fn }

// SetSettingsFn регистрирует функцию для /settings.
func (c *CommandHandler) SetSettingsFn(fn func() string) { c.settingsFn = fn }

// StartListening запускает поллинг и обрабатывает команды.
// Принимает команды из ЛЮБОГО чата (личка с ботом, группа).
// Ответ отправляется в тот же чат откуда пришла команда.
func (c *CommandHandler) StartListening() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30
	// Получаем только сообщения (команды) — не загружаем другие типы апдейтов
	u.AllowedUpdates = []string{"message"}

	updates := c.bot.GetUpdatesChan(u)
	c.log.Info("Telegram: поллинг команд запущен, пишите /help боту в личку")

	for update := range updates {
		if update.Message == nil {
			continue
		}

		// Логируем все входящие сообщения для диагностики
		c.log.Info("Telegram входящее сообщение",
			zap.String("from", update.Message.From.UserName),
			zap.Int64("chat_id", update.Message.Chat.ID),
			zap.String("chat_type", update.Message.Chat.Type),
			zap.String("text", update.Message.Text),
		)

		if !update.Message.IsCommand() {
			continue
		}

		chatID := update.Message.Chat.ID
		cmd := update.Message.Command()
		start := time.Now()

		c.log.Info("Telegram команда получена",
			zap.String("cmd", "/"+cmd),
			zap.String("from", update.Message.From.UserName),
			zap.Int64("chat_id", chatID),
		)

		c.chatID = chatID // сохраняем для отправки фото
		reply := c.handle(cmd)
		elapsed := time.Since(start)

		c.log.Info("команда обработана",
			zap.String("cmd", "/"+cmd),
			zap.Duration("elapsed", elapsed),
		)

		// Добавляем время выполнения в конец ответа
		reply += fmt.Sprintf("\n\n<i>⏱ %s</i>", fmtDuration(elapsed))

		if err := c.sender.Reply(chatID, reply); err != nil {
			c.log.Error("ошибка отправки ответа", zap.Error(err))
		}
	}
}

func (c *CommandHandler) handle(cmd string) string {
	switch cmd {
	case "start", "help":
		return helpText()

	case "status":
		if c.statusFn != nil {
			return c.statusFn()
		}
		return "Статус недоступен"

	case "pause":
		if c.pauseFn != nil {
			c.pauseFn()
		}
		return "⏸ <b>Сканер приостановлен</b>\nАвтоматические сигналы временно отключены.\nОтправьте /resume чтобы возобновить."

	case "resume":
		if c.resumeFn != nil {
			c.resumeFn()
		}
		return "▶️ <b>Сканер возобновлён</b>"

	case "scan":
		return c.handleScan()

	case "top":
		return c.handleTop()

	case "settings":
		if c.settingsFn != nil {
			return c.settingsFn()
		}
		return "Настройки недоступны"

	default:
		return fmt.Sprintf("Неизвестная команда /%s\n\n%s", cmd, helpText())
	}
}

func (c *CommandHandler) handleScan() string {
	if c.scanFn == nil {
		return "⚠️ Сканирование недоступно"
	}
	result := c.scanFn()
	if result == nil {
		return "⚠️ Сканер не отвечает"
	}

	source := "🔄 свежий скан"
	if result.FromCache {
		source = fmt.Sprintf("📦 кеш (%s назад)", fmtDuration(result.CacheAge.Round(time.Second)))
	}

	// ── PNG-график для монеты с наивысшим рейтингом — отправляем как фото ──
	if len(result.ChartBytes) > 0 && c.chatID != 0 {
		caption := result.ChartCaption
		if caption == "" {
			caption = fmt.Sprintf(
				"🔍 <b>Лучший сетап</b> — %s\nScore: <b>%.0f/100</b> %s",
				source, result.BestScore, result.BestGrade,
			)
		}
		_ = c.sender.SendPhotoToChat(c.chatID, result.ChartBytes, caption)

		// После фото — краткое дополнение
		if result.SignalsFound > 0 {
			return fmt.Sprintf("✅ Квалифицированных сигналов: <b>%d</b>", result.SignalsFound)
		}
		return fmt.Sprintf("🔍 <b>Проверено:</b> %d монет  |  %s", result.ScannedCount, source)
	}

	// ── Fallback: только текст если chart не был сгенерирован ──
	sb := strings.Builder{}
	fmt.Fprintf(&sb, "🔍 <b>Сканирование</b> — %s | %d монет\n\n", source, result.ScannedCount)
	if result.BestSetup != "" {
		fmt.Fprintf(&sb, "🏆 Score: <b>%.0f/100</b> %s\n%s\n\n",
			result.BestScore, result.BestGrade, result.BestSetup)
	}
	if result.SignalsFound > 0 {
		fmt.Fprintf(&sb, "✅ Сигналов: <b>%d</b>\n\n%s", result.SignalsFound, result.BestSignal)
	} else {
		fmt.Fprintf(&sb, "⬇️ <b>NO TRADE</b> по фильтрам")
	}
	return sb.String()
}

func (c *CommandHandler) handleTop() string {
	if c.scanFn == nil {
		return "⚠️ Данные недоступны"
	}
	result := c.scanFn()
	if result == nil || len(result.TopCoins) == 0 {
		return "⚠️ Данные рейтинга недоступны"
	}
	return "📊 <b>Топ монет по Market Opportunity Score</b>\n\n" +
		formatTopCoins(result.TopCoins, 10)
}

func formatTopCoins(coins []CoinRank, limit int) string {
	if len(coins) == 0 {
		return "Данные отсутствуют"
	}
	if limit > len(coins) {
		limit = len(coins)
	}
	medals := []string{"🥇", "🥈", "🥉", "4.", "5.", "6.", "7.", "8.", "9.", "10."}
	sb := strings.Builder{}
	for i, coin := range coins[:limit] {
		medal := fmt.Sprintf("%d.", i+1)
		if i < len(medals) {
			medal = medals[i]
		}
		dir := ""
		switch coin.Direction {
		case "Long":
			dir = " 🟢"
		case "Short":
			dir = " 🔴"
		}
		fmt.Fprintf(&sb, "%s <b>%s</b>%s — <code>%.1f</code>\n",
			medal, coin.Symbol, dir, coin.Score)
	}
	return sb.String()
}

// fmtDuration форматирует время выполнения в читаемый вид.
func fmtDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

func helpText() string {
	return `🤖 <b>BYBIT FUTURES ELITE SIGNAL ENGINE</b>

<b>Команды:</b>
/scan      — немедленное сканирование + лучший сетап
/top       — рейтинг топ монет прямо сейчас
/status    — аптайм, счётчики сигналов
/settings  — текущие параметры системы
/pause     — приостановить автосигналы
/resume    — возобновить автосигналы
/help      — это сообщение

<i>Команды принимаются в личном чате с ботом или в группе.
Сигналы отправляются в настроенный канал/чат.</i>`
}
