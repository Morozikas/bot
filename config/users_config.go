// Пакет config — конфигурация системы.
// users_config.go — загрузка конфигурации пользователей из .env или users.json.
// Поддерживает до 10 пользователей через переменные USER_N_*.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

// UserConfig — настройки одного торгового аккаунта.
type UserConfig struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	APIKey    string  `json:"api_key"`
	APISecret string  `json:"api_secret"`
	Testnet   bool    `json:"testnet"`

	RiskPct      float64        `json:"risk_pct"`
	MaxLeverage  int            `json:"max_leverage"`
	MaxPositions int            `json:"max_positions"`

	// Плечо по символу: {"BTCUSDT": 20}
	SymbolLeverage map[string]int `json:"symbol_leverage"`

	// Разрешённые символы (пусто = все)
	AllowedSymbols []string `json:"allowed_symbols"`

	// Минимальный Score для исполнения сигнала у этого пользователя
	MinScore float64 `json:"min_score"`

	Active bool `json:"active"`
}

// LoadUsersFromEnv загружает пользователей из переменных окружения .env.
// Формат: USER_1_ID, USER_1_API_KEY, USER_1_API_SECRET, ...
func LoadUsersFromEnv() []UserConfig {
	users := make([]UserConfig, 0)
	for i := 1; i <= 10; i++ {
		prefix := fmt.Sprintf("USER_%d_", i)
		id := os.Getenv(prefix + "ID")
		if id == "" {
			continue
		}
		testnet, _ := strconv.ParseBool(os.Getenv(prefix + "TESTNET"))
		riskPct, _ := strconv.ParseFloat(getEnv(prefix+"RISK_PCT", "1.0"), 64)
		maxLev, _ := strconv.Atoi(getEnv(prefix+"MAX_LEVERAGE", "10"))
		maxPos, _ := strconv.Atoi(getEnv(prefix+"MAX_POSITIONS", "5"))
		minScore, _ := strconv.ParseFloat(getEnv(prefix+"MIN_SCORE", "90"), 64)
		active, _ := strconv.ParseBool(getEnv(prefix+"ACTIVE", "true"))

		users = append(users, UserConfig{
			ID:           id,
			Name:         getEnv(prefix+"NAME", id),
			APIKey:       os.Getenv(prefix + "API_KEY"),
			APISecret:    os.Getenv(prefix + "API_SECRET"),
			Testnet:      testnet,
			RiskPct:      riskPct,
			MaxLeverage:  maxLev,
			MaxPositions: maxPos,
			MinScore:     minScore,
			Active:       active,
			SymbolLeverage: make(map[string]int),
		})
	}
	return users
}

// LoadUsersFromFile загружает пользователей из JSON-файла.
func LoadUsersFromFile(path string) ([]UserConfig, error) {
	if path == "" {
		path = "users.json"
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []UserConfig{}, nil
		}
		return nil, err
	}
	defer f.Close()
	var users []UserConfig
	return users, json.NewDecoder(f).Decode(&users)
}

// LoadUsers загружает пользователей: сначала из .env, затем из users.json (объединяет).
func LoadUsers() []UserConfig {
	envUsers := LoadUsersFromEnv()
	fileUsers, _ := LoadUsersFromFile("users.json")
	seen := make(map[string]bool)
	result := make([]UserConfig, 0, len(envUsers)+len(fileUsers))
	for _, u := range envUsers {
		result = append(result, u)
		seen[u.ID] = true
	}
	for _, u := range fileUsers {
		if !seen[u.ID] {
			result = append(result, u)
		}
	}
	return result
}

// SaveUsersToFile сохраняет конфигурацию пользователей в JSON-файл.
func SaveUsersToFile(users []UserConfig, path string) error {
	if path == "" {
		path = "users.json"
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(users)
}
