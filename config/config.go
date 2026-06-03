// Пакет config — конфигурация системы.
// config.go — загрузка всех настроек из .env, применение дефолтов.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// AppConfig — полная конфигурация приложения.
type AppConfig struct {
	Bybit      BybitConfig
	Telegram   TelegramConfig
	Database   DatabaseConfig
	Redis      RedisConfig
	Network    NetworkConfig
	Signal     SignalConfig
	Trading    TradingConfig
	Monitoring MonitoringConfig
	Log        LogConfig
}

type BybitConfig struct {
	APIKey    string
	APISecret string
	Testnet   bool
	BaseURL   string
	WSURL     string
}

type TelegramConfig struct {
	BotToken    string
	ChatID      string
	ExtraChatIDs []int64 // дополнительные каналы для рассылки сигналов
}

type DatabaseConfig struct{ URL string }
type RedisConfig struct{ URL string }

type NetworkConfig struct {
	ProxyURL string
	Timeout  time.Duration
}

type LogConfig struct{ Level string }
type MonitoringConfig struct{ Port int }

// Load читает .env и заполняет AppConfig.
func Load() (*AppConfig, error) {
	if err := godotenv.Load(); err != nil {
		// .env не найден — переменные берутся из окружения системы (или пусты)
		fmt.Fprintln(os.Stderr, "⚠️  ВНИМАНИЕ: файл .env не найден — создайте его из .env.example")
	}
	cfg := &AppConfig{}

	testnet, _ := strconv.ParseBool(getEnv("BYBIT_TESTNET", "false"))
	cfg.Bybit = BybitConfig{
		APIKey:    os.Getenv("BYBIT_API_KEY"),
		APISecret: os.Getenv("BYBIT_API_SECRET"),
		Testnet:   testnet,
	}
	if testnet {
		cfg.Bybit.BaseURL = "https://api-testnet.bybit.com"
		cfg.Bybit.WSURL = "wss://stream-testnet.bybit.com/v5/public/linear"
	} else {
		cfg.Bybit.BaseURL = "https://api.bybit.com"
		cfg.Bybit.WSURL = "wss://stream.bybit.com/v5/public/linear"
	}

	cfg.Telegram = TelegramConfig{
		BotToken:     os.Getenv("TELEGRAM_BOT_TOKEN"),
		ChatID:       os.Getenv("TELEGRAM_CHAT_ID"),
		ExtraChatIDs: parseExtraChatIDs(os.Getenv("TELEGRAM_EXTRA_CHATS")),
	}

	// PostgreSQL подключается ТОЛЬКО если явно задан DATABASE_URL — иначе пропускается
	cfg.Database = DatabaseConfig{URL: os.Getenv("DATABASE_URL")}
	// Redis подключается ТОЛЬКО если явно задан REDIS_URL — иначе пропускается
	cfg.Redis = RedisConfig{URL: os.Getenv("REDIS_URL")}
	cfg.Network = NetworkConfig{ProxyURL: os.Getenv("PROXY_URL"), Timeout: 10 * time.Second}
	cfg.Log = LogConfig{Level: getEnv("LOG_LEVEL", "INFO")}

	port, _ := strconv.Atoi(getEnv("APP_PORT", "8080"))
	cfg.Monitoring = MonitoringConfig{Port: port}

	cfg.Signal = DefaultSignalConfig()
	cfg.Trading = DefaultTradingConfig()

	// ---- Переопределение параметров из .env ----

	// Торговля
	cfg.Trading.Enabled, _ = strconv.ParseBool(os.Getenv("TRADING_ENABLED"))

	// Тайминги сканирования
	if v, ok := envInt("SCAN_INTERVAL_SEC"); ok && v > 0 {
		cfg.Signal.ScanIntervalSec = v
	}
	if v, ok := envInt("FAST_DATA_INTERVAL_SEC"); ok && v > 0 {
		cfg.Signal.FastDataIntervalSec = v
	}
	if v, ok := envInt("CANDLES_1M_INTERVAL_SEC"); ok && v > 0 {
		cfg.Signal.Candles1MCandleIntervalSec = v
	}
	if v, ok := envInt("CANDLES_5M_INTERVAL_SEC"); ok && v > 0 {
		cfg.Signal.Candles5MCandleIntervalSec = v
	}
	if v, ok := envInt("CANDLES_15M_INTERVAL_SEC"); ok && v > 0 {
		cfg.Signal.Candles15MCandleIntervalSec = v
	}
	if v, ok := envInt("CANDLES_HTF_INTERVAL_SEC"); ok && v > 0 {
		cfg.Signal.CandlesHTFIntervalSec = v
	}
	if v, ok := envInt("DERIVATIVES_INTERVAL_SEC"); ok && v > 0 {
		cfg.Signal.DerivativesIntervalSec = v
	}

	// Количество сканируемых монет
	if v, ok := envInt("TOP_COINS_LIMIT"); ok && v > 0 {
		cfg.Signal.TopCoinsLimit = v
	}
	if v, ok := envInt("ANALYSIS_LIMIT"); ok && v > 0 {
		cfg.Signal.AnalysisLimit = v
	}
	if v, ok := envInt("WORKER_COUNT"); ok && v > 0 {
		cfg.Signal.WorkerCount = v
	}

	// Пороги качества сигналов
	if v, ok := envFloat("MIN_SCORE_PUBLISH"); ok && v > 0 {
		cfg.Signal.MinScorePublish = v
	}
	if v, ok := envFloat("MIN_SCORE_APLUS"); ok && v > 0 {
		cfg.Signal.MinScoreAPlus = v
	}
	if v, ok := envFloat("MIN_SCORE_A"); ok && v > 0 {
		cfg.Signal.MinScoreA = v
	}
	if v, ok := envFloat("MIN_SCORE_B"); ok && v > 0 {
		cfg.Signal.MinScoreB = v
	}
	if v, ok := envFloat("MIN_RR"); ok && v > 0 {
		cfg.Signal.MinRR = v
	}
	if v, ok := envFloat("MIN_PROBABILITY"); ok && v > 0 {
		cfg.Signal.MinProbability = v
	}

	// Веса скоринга
	if v, ok := envFloat("WEIGHT_LIQUIDITY"); ok && v > 0 {
		cfg.Signal.LiquidityWeight = v
	}
	if v, ok := envFloat("WEIGHT_STRUCTURE"); ok && v > 0 {
		cfg.Signal.StructureWeight = v
	}
	if v, ok := envFloat("WEIGHT_FVGOB"); ok && v > 0 {
		cfg.Signal.FVGOBWeight = v
	}
	if v, ok := envFloat("WEIGHT_OI"); ok && v > 0 {
		cfg.Signal.OIWeight = v
	}
	if v, ok := envFloat("WEIGHT_VOLUME"); ok && v > 0 {
		cfg.Signal.VolumeWeight = v
	}

	// Чувствительность антиспуфинга и прочие фильтры
	if v, ok := envFloat("SPOOFING_SENSITIVITY"); ok && v > 0 {
		cfg.Signal.SpoofingSensitivity = v
	}
	if v, ok := envFloat("MIN_VOLUME_RATIO"); ok && v > 0 {
		cfg.Signal.MinVolumeRatio = v
	}

	// Order Block фильтры
	if v, ok := envFloat("OB_MIN_FRESHNESS"); ok {
		cfg.Signal.OBMinFreshness = v
	}
	if v, ok := envFloat("OB_MIN_VOLUME_STRENGTH"); ok && v > 0 {
		cfg.Signal.OBMinVolumeStrength = v
	}
	if v, err := strconv.Atoi(os.Getenv("OB_MAX_RETEST_COUNT")); err == nil {
		cfg.Signal.OBMaxRetestCount = v
	}
	if v, ok := envFloat("OB_MIN_DISPLACEMENT"); ok && v > 0 {
		cfg.Signal.OBMinDisplacement = v
	}

	// Режим торговли и тейки
	if v := os.Getenv("TRADE_MODE"); v != "" {
		cfg.Signal.TradeMode = v
	}
	if v, err := strconv.Atoi(os.Getenv("TP_COUNT")); err == nil && v >= 1 && v <= 3 {
		cfg.Signal.TPCount = v
	}

	// Сопровождение
	if v, ok := envBool("TRADE_MANAGEMENT"); ok {
		cfg.Signal.TradeManagement = v
	}
	if v, ok := envFloat("BREAKEVEN_AT_RR"); ok && v > 0 {
		cfg.Signal.BreakevenAtRR = v
	}
	if v, ok := envBool("NOTIFY_ON_TP"); ok {
		cfg.Signal.NotifyOnTP = v
	}
	if v, ok := envBool("NOTIFY_ON_SL"); ok {
		cfg.Signal.NotifyOnSL = v
	}
	if v, ok := envBool("NOTIFY_ON_BE"); ok {
		cfg.Signal.NotifyOnBE = v
	}

	// Pending зоны
	if v, ok := envInt("PENDING_ZONE_CHECK_SEC"); ok && v > 0 {
		cfg.Signal.PendingZoneCheckSec = v
	}
	if v, ok := envInt("PENDING_ZONE_MAX_AGE_SEC"); ok && v > 0 {
		cfg.Signal.PendingZoneMaxAgeSec = v
	}

	return cfg, nil
}

func parseExtraChatIDs(s string) []int64 {
	if s == "" {
		return nil
	}
	result := make([]int64, 0)
	for _, part := range splitComma(s) {
		if id, err := strconv.ParseInt(part, 10, 64); err == nil {
			result = append(result, id)
		}
	}
	return result
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// cleanEnvVal обрезает пробелы и инлайн-комментарии из значения .env.
// Решает проблему: SCAN_INTERVAL_SEC=600      # комментарий → "600      "
// strconv.Atoi("600      ") возвращает ошибку, поэтому обрезаем перед парсингом.
func cleanEnvVal(key string) string {
	v := os.Getenv(key)
	// Обрезаем пробелы
	v = strings.TrimSpace(v)
	// Убираем инлайн-комментарий если есть
	if idx := strings.Index(v, "#"); idx >= 0 {
		v = strings.TrimSpace(v[:idx])
	}
	return v
}

// envInt читает int из .env с корректной обрезкой комментариев.
func envInt(key string) (int, bool) {
	v := cleanEnvVal(key)
	if v == "" {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	return n, err == nil
}

// envFloat читает float64 из .env с корректной обрезкой комментариев.
func envFloat(key string) (float64, bool) {
	v := cleanEnvVal(key)
	if v == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(v, 64)
	return f, err == nil
}

// envBool читает bool из .env с корректной обрезкой комментариев.
func envBool(key string) (bool, bool) {
	v := cleanEnvVal(key)
	if v == "" {
		return false, false
	}
	b, err := strconv.ParseBool(v)
	return b, err == nil
}

func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	parts := make([]string, 0)
	cur := ""
	for _, c := range s {
		if c == ',' {
			if cur != "" {
				parts = append(parts, cur)
			}
			cur = ""
		} else if c != ' ' {
			cur += string(c)
		}
	}
	if cur != "" {
		parts = append(parts, cur)
	}
	return parts
}
