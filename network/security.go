// Пакет network — работа с сетью.
// security.go — базовые проверки безопасности: валидация ключей, санитизация строк.
package network

import (
	"fmt"
	"regexp"
	"strings"
)

var apiKeyRegex = regexp.MustCompile(`^[A-Za-z0-9_-]{10,64}$`)

// ValidateAPIKey проверяет формат API-ключа Bybit.
func ValidateAPIKey(key string) error {
	if key == "" {
		return fmt.Errorf("API ключ не задан")
	}
	if !apiKeyRegex.MatchString(key) {
		return fmt.Errorf("недопустимый формат API ключа")
	}
	return nil
}

// SanitizeSymbol очищает символ от посторонних символов.
func SanitizeSymbol(sym string) string {
	return strings.ToUpper(strings.TrimSpace(sym))
}

// MaskKey маскирует секретный ключ для логирования.
func MaskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}
