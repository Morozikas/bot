// Пакет telegram — интеграция с Telegram.
// bot.go — высокоуровневый интерфейс: стартовое уведомление, рассылка сигналов.
package telegram

import (
	"fmt"
	"math"
	"strings"
	"time"

	signalengine "bybit-elite-signal/signal_engine"

	"go.uber.org/zap"
)

// Bot управляет отправкой уведомлений.
type Bot struct {
	sender       *Sender
	extraChatIDs []int64
	log          *zap.Logger
}

// NewBot создаёт Bot и сразу отправляет стартовое сообщение в основной чат.
func NewBot(sender *Sender, extraChatIDs []int64, log *zap.Logger) *Bot {
	b := &Bot{sender: sender, extraChatIDs: extraChatIDs, log: log}
	b.sendStartupMessage()
	return b
}

// sendStartupMessage отправляет уведомление о запуске с инструкцией по командам.
func (b *Bot) sendStartupMessage() {
	username := b.sender.Bot().Self.UserName
	msg := fmt.Sprintf(
		"🚀 <b>BYBIT FUTURES ELITE SIGNAL ENGINE запущен</b>\n\n"+
			"Бот активен и начинает сканирование рынка.\n\n"+
			"📋 <b>Управление ботом:</b>\n"+
			"Напишите в <b>личку боту</b> @%s\n"+
			"или добавьте бота в <b>группу</b> и пишите там.\n\n"+
			"<b>Доступные команды:</b>\n"+
			"/scan — немедленное сканирование\n"+
			"/top — рейтинг монет\n"+
			"/status — статус работы\n"+
			"/settings — параметры системы\n"+
			"/pause — пауза / /resume — продолжить\n"+
			"/help — все команды\n\n"+
			"<i>%s UTC</i>",
		username,
		time.Now().UTC().Format("02.01.2006 15:04:05"),
	)
	if err := b.sender.SendHTML(msg); err != nil {
		b.log.Error("не удалось отправить стартовое сообщение", zap.Error(err))
	} else {
		b.log.Info("стартовое сообщение отправлено в Telegram")
	}
}

// NewCommandHandler создаёт обработчик команд.
func (b *Bot) NewCommandHandler(log *zap.Logger, statusFn func() string, pauseFn, resumeFn func()) *CommandHandler {
	return NewCommandHandler(b.sender, log, statusFn, pauseFn, resumeFn)
}

// NotifySignal отправляет сигнал в основной чат и все дополнительные каналы.
func (b *Bot) NotifySignal(sig *signalengine.Signal) {
	text := FormatSignal(sig)
	if err := b.sender.SendHTML(text); err != nil {
		b.log.Error("ошибка отправки сигнала", zap.Error(err))
	}
	for _, chatID := range b.extraChatIDs {
		if err := b.sender.SendSignalToChat(chatID, text); err != nil {
			b.log.Error("ошибка отправки в канал", zap.Int64("chat_id", chatID), zap.Error(err))
		}
	}
}

// NotifySignalWithChart отправляет PNG-график + краткую подпись.
// Используется вместо NotifySignal когда доступен chart.
func (b *Bot) NotifySignalWithChart(pngBytes []byte, caption string) {
	if err := b.sender.SendPhoto(pngBytes, caption); err != nil {
		b.log.Error("ошибка отправки графика", zap.Error(err))
	}
	for _, chatID := range b.extraChatIDs {
		if err := b.sender.SendPhotoToChat(chatID, pngBytes, caption); err != nil {
			b.log.Error("ошибка отправки графика в канал", zap.Int64("chat_id", chatID), zap.Error(err))
		}
	}
}

// NotifyInfo отправляет информационное сообщение в основной чат.
func (b *Bot) NotifyInfo(msg string) { _ = b.sender.SendText(msg) }

// NotifyError отправляет уведомление об ошибке.
func (b *Bot) NotifyError(msg string) { _ = b.sender.SendText("⚠️ ОШИБКА: " + msg) }

// NotifyExecution отправляет результаты автоторговли.
func (b *Bot) NotifyExecution(msg string) {
	if msg != "" {
		_ = b.sender.SendHTML(msg)
	}
}

// FormatSignal форматирует сигнал для Telegram.
func FormatSignal(sig *signalengine.Signal) string {
	emoji, dir := "🟢", "LONG"
	if sig.Direction == "Short" {
		emoji, dir = "🔴", "SHORT"
	}
	stars := map[string]string{"A+": "⭐⭐⭐", "A": "⭐⭐", "B": "⭐"}[string(sig.Grade)]
	return fmt.Sprintf(
		"<b>%s %s %s</b>\n\n"+
			"<b>ENTRY:</b>  <code>%s</code>\n"+
			"<b>STOP LOSS:</b>  <code>%s</code>\n"+
			"<b>TAKE PROFIT:</b>  <code>%s</code>\n\n"+
			"<b>RR:</b>  1:%.1f\n"+
			"<b>PROBABILITY:</b>  %.0f%%\n"+
			"<b>SCORE:</b>  %.0f/100 %s\n\n"+
			"<b>SETUP:</b>\n<i>%s</i>\n\n"+
			"<code>%s UTC</code>\n⸻",
		emoji, dir, sig.Symbol,
		fmtP(sig.Entry), fmtP(sig.StopLoss), fmtP(sig.TakeProfit),
		sig.RR, sig.Probability, sig.Score, stars, sig.Setup,
		time.Now().UTC().Format("02.01.2006 15:04:05"),
	)
}

// FormatSignalFull форматирует сигнал с несколькими TP, режимом и дедлайном.
func FormatSignalFull(sig *signalengine.Signal, tps []float64, modeLabel, deadline string) string {
	emoji, dir := "🟢", "LONG"
	if sig.Direction == "Short" {
		emoji, dir = "🔴", "SHORT"
	}
	stars := map[string]string{"A+": "⭐⭐⭐", "A": "⭐⭐", "B": "⭐"}[string(sig.Grade)]

	tpLines := ""
	for i, tp := range tps {
		pct := math.Abs(tp-sig.Entry) / sig.Entry * 100
		arrow := "↑"
		if sig.Direction == "Short" {
			arrow = "↓"
		}
		tpLines += fmt.Sprintf("<b>TP%d:</b>  <code>%s</code>  %s<i>+%.2f%%</i>\n",
			i+1, fmtP(tp), arrow, pct)
	}

	slPct := math.Abs(sig.Entry-sig.StopLoss) / sig.Entry * 100

	return fmt.Sprintf(
		"%s <b>%s %s</b>  |  %s\n\n"+
			"<b>ENTRY:</b>  <code>%s</code>\n"+
			"<b>STOP:</b>   <code>%s</code>  ↓<i>−%.2f%%</i>\n\n"+
			"%s\n"+
			"<b>RR:</b>  1:%.1f  |  <b>Score:</b>  %.0f/100 %s\n"+
			"<b>Prob:</b>  %.0f%%\n\n"+
			"<b>Setup:</b> <i>%s</i>\n\n"+
			"⏰ <b>%s</b>  |  <code>%s UTC</code>\n⸻",
		emoji, dir, sig.Symbol, modeLabel,
		fmtP(sig.Entry),
		fmtP(sig.StopLoss), slPct,
		tpLines,
		sig.RR, sig.Score, stars,
		sig.Probability,
		sig.Setup,
		deadline,
		time.Now().UTC().Format("02.01.2006 15:04:05"),
	)
}

func fmtP(p float64) string {
	d := 2
	if p >= 10000 {
		d = 1
	} else if p < 1 {
		d = 6
	}
	return strings.TrimRight(strings.TrimRight(
		fmt.Sprintf("%."+fmt.Sprintf("%d", d)+"f", math.Round(p*math.Pow10(d))/math.Pow10(d)),
		"0"), ".")
}
