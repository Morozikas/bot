// Пакет telegram — интеграция с Telegram.
// sender.go — инициализация бота: удаление webhook, настройка поллинга, отправка сообщений.
package telegram

import (
	"fmt"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

// Sender отправляет сообщения через Telegram Bot API.
type Sender struct {
	bot    *tgbotapi.BotAPI
	chatID int64 // основной чат / канал для сигналов
	log    *zap.Logger
}

// NewSender создаёт Sender, удаляет вебхук и проверяет подключение.
func NewSender(token, chatIDStr string, log *zap.Logger) (*Sender, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("инициализация Telegram: %w", err)
	}

	// Удаляем вебхук — обязательно, иначе GetUpdatesChan ничего не получает.
	// DropPendingUpdates: false — не теряем команды, отправленные пока бот был выключен.
	if _, err := bot.Request(tgbotapi.DeleteWebhookConfig{DropPendingUpdates: false}); err != nil {
		log.Warn("не удалось удалить webhook", zap.Error(err))
	} else {
		log.Info("webhook удалён, поллинг активен")
	}

	// Включаем debug-лог только если уровень DEBUG — видим все входящие апдейты
	// bot.Debug = true  // раскомментируйте для отладки

	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("неверный TELEGRAM_CHAT_ID %q: %w", chatIDStr, err)
	}

	log.Info("Telegram бот подключён",
		zap.String("username", "@"+bot.Self.UserName),
		zap.Int64("chat_id", chatID),
	)

	return &Sender{bot: bot, chatID: chatID, log: log}, nil
}

// SendText отправляет обычный текст в основной чат.
func (s *Sender) SendText(text string) error {
	return s.sendTo(s.chatID, text, "")
}

// SendHTML отправляет HTML-сообщение в основной чат.
func (s *Sender) SendHTML(html string) error {
	return s.sendTo(s.chatID, html, tgbotapi.ModeHTML)
}

// SendToChat отправляет сообщение в произвольный чат.
func (s *Sender) SendToChat(chatID int64, text string) error {
	return s.sendTo(chatID, text, "")
}

// SendSignalToChat отправляет HTML-сигнал в указанный чат.
func (s *Sender) SendSignalToChat(chatID int64, signalText string) error {
	return s.sendTo(chatID, signalText, tgbotapi.ModeHTML)
}

// Reply отвечает в конкретный чат (используется для ответа на команды).
func (s *Sender) Reply(chatID int64, text string) error {
	return s.sendTo(chatID, text, tgbotapi.ModeHTML)
}

func (s *Sender) sendTo(chatID int64, text, parseMode string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	if parseMode != "" {
		msg.ParseMode = parseMode
	}
	msg.DisableWebPagePreview = true
	if _, err := s.bot.Send(msg); err != nil {
		s.log.Error("ошибка отправки в Telegram",
			zap.Int64("chat_id", chatID),
			zap.Error(err))
		return err
	}
	return nil
}

// SendPhoto отправляет PNG-изображение с подписью в основной чат.
func (s *Sender) SendPhoto(pngBytes []byte, caption string) error {
	reader := tgbotapi.FileBytes{Name: "signal.png", Bytes: pngBytes}
	msg := tgbotapi.NewPhoto(s.chatID, reader)
	msg.Caption = caption
	msg.ParseMode = tgbotapi.ModeHTML
	_, err := s.bot.Send(msg)
	if err != nil {
		s.log.Error("ошибка отправки фото", zap.Error(err))
	}
	return err
}

// SendPhotoToChat отправляет PNG в произвольный чат.
func (s *Sender) SendPhotoToChat(chatID int64, pngBytes []byte, caption string) error {
	reader := tgbotapi.FileBytes{Name: "signal.png", Bytes: pngBytes}
	msg := tgbotapi.NewPhoto(chatID, reader)
	msg.Caption = caption
	msg.ParseMode = tgbotapi.ModeHTML
	_, err := s.bot.Send(msg)
	return err
}

// Bot возвращает внутренний объект бота.
func (s *Sender) Bot() *tgbotapi.BotAPI { return s.bot }

// DefaultChatID возвращает основной chatID.
func (s *Sender) DefaultChatID() int64 { return s.chatID }
