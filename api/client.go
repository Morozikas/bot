// Пакет api — взаимодействие с Bybit V5 API.
// client.go — HTTP-клиент с защитой от банов:
//   - токен-бакет 80 req/min (запас 33% от лимита Bybit)
//   - случайный джиттер между запросами (50–300 мс)
//   - экспоненциальный бэкофф при 429 (1s → 2s → 4s → 8s → 16s → стоп)
//   - чтение X-BAPI-LIMIT-STATUS из ответа → автоматическое замедление
//   - circuit breaker: 3 подряд 429 → пауза 60 секунд
//   - обработка кодов ошибок Bybit: 10006 (IP ban), 10018 (rate limit)
package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"sort"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ---- Rate Limiter ----

type rateLimiter struct {
	mu       sync.Mutex
	tokens   float64
	capacity float64
	refillPS float64
	lastTime time.Time
}

// requestDelay — фиксированная задержка между запросами (из BYBIT_REQUEST_DELAY_MS).
var requestDelay = func() time.Duration {
	v := strings.TrimSpace(os.Getenv("BYBIT_REQUEST_DELAY_MS"))
	if idx := strings.Index(v, "#"); idx >= 0 {
		v = strings.TrimSpace(v[:idx])
	}
	if ms, err := strconv.Atoi(v); err == nil && ms >= 0 {
		return time.Duration(ms) * time.Millisecond
	}
	return 500 * time.Millisecond // default 500 мс
}()

func newRateLimiter() *rateLimiter {
	v := strings.TrimSpace(os.Getenv("BYBIT_RATE_LIMIT"))
	if idx := strings.Index(v, "#"); idx >= 0 {
		v = strings.TrimSpace(v[:idx])
	}
	ratePerMin := 30.0
	if n, err := strconv.ParseFloat(v, 64); err == nil && n > 0 && n <= 100 {
		ratePerMin = n
	}
	burst := 2.0
	if ratePerMin > 50 {
		burst = 4
	}
	return &rateLimiter{
		tokens:   burst,
		capacity: burst,
		refillPS: ratePerMin / 60.0,
		lastTime: time.Now(),
	}
}

func (r *rateLimiter) wait() {
	for {
		r.mu.Lock()
		now := time.Now()
		r.tokens += now.Sub(r.lastTime).Seconds() * r.refillPS
		r.lastTime = now
		if r.tokens > r.capacity {
			r.tokens = r.capacity
		}
		if r.tokens >= 1 {
			r.tokens--
			r.mu.Unlock()
			return
		}
		r.mu.Unlock()
		time.Sleep(20 * time.Millisecond)
	}
}

// slowDown снижает скорость заполнения токенов на N% на 30 секунд.
// Вызывается когда Bybit сообщает что лимит почти исчерпан.
func (r *rateLimiter) slowDown(pct float64) {
	r.mu.Lock()
	original := r.refillPS
	r.refillPS = original * (1.0 - pct)
	r.mu.Unlock()
	time.AfterFunc(30*time.Second, func() {
		r.mu.Lock()
		r.refillPS = original
		r.mu.Unlock()
	})
}

// ---- Circuit Breaker ----

type circuitBreaker struct {
	failures  atomic.Int32 // последовательных 429
	openUntil atomic.Int64 // unix nano до которого схема "открыта" (блокировка)
	threshold int32
	cooldown  time.Duration
}

func newCircuitBreaker() *circuitBreaker {
	return &circuitBreaker{threshold: 3, cooldown: 60 * time.Second}
}

// isOpen возвращает true если нужно подождать перед следующим запросом.
func (cb *circuitBreaker) isOpen() (bool, time.Duration) {
	until := cb.openUntil.Load()
	if until == 0 {
		return false, 0
	}
	remaining := time.Until(time.Unix(0, until))
	if remaining <= 0 {
		cb.openUntil.Store(0)
		cb.failures.Store(0)
		return false, 0
	}
	return true, remaining
}

func (cb *circuitBreaker) recordFailure() {
	n := cb.failures.Add(1)
	if n >= cb.threshold {
		cb.openUntil.Store(time.Now().Add(cb.cooldown).UnixNano())
	}
}

func (cb *circuitBreaker) recordSuccess() {
	cb.failures.Store(0)
}

// ---- Client ----

// banState — глобальное состояние IP-бана (все клиенты разделяют одно состояние).
var (
	banUntil     atomic.Int64 // unix nano
	banNotified  atomic.Bool
)

// Client — HTTP-клиент для Bybit REST API с защитой от банов.
type Client struct {
	apiKey    string
	apiSecret string
	baseURL   string
	http      *http.Client
	limiter   *rateLimiter
	breaker   *circuitBreaker
	onBan     func(duration time.Duration) // коллбэк при обнаружении бана
}

// NewClient создаёт REST-клиент. Без ключей — только публичные эндпоинты.
func NewClient(baseURL, apiKey, apiSecret, proxyURL string) (*Client, error) {
	t := &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     90 * time.Second,
	}
	if proxyURL != "" {
		pu, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("прокси URL: %w", err)
		}
		t.Proxy = http.ProxyURL(pu)
	}
	return &Client{
		apiKey:    apiKey,
		apiSecret: apiSecret,
		baseURL:   baseURL,
		http:      &http.Client{Timeout: 12 * time.Second, Transport: t},
		limiter:   newRateLimiter(),
		breaker:   newCircuitBreaker(),
	}, nil
}

// IsAuth возвращает true если клиент аутентифицирован.
func (c *Client) IsAuth() bool { return c.apiKey != "" && c.apiSecret != "" }

// SetBanCallback регистрирует функцию уведомления при IP-бане.
func (c *Client) SetBanCallback(fn func(duration time.Duration)) { c.onBan = fn }

// IsBanned возвращает true и оставшееся время если IP заблокирован.
func IsBanned() (bool, time.Duration) {
	until := banUntil.Load()
	if until == 0 {
		return false, 0
	}
	remaining := time.Until(time.Unix(0, until))
	if remaining <= 0 {
		banUntil.Store(0)
		banNotified.Store(false)
		return false, 0
	}
	return true, remaining
}

// triggerBan активирует глобальную заморозку запросов.
func triggerBan(duration time.Duration, onBan func(time.Duration)) {
	banUntil.Store(time.Now().Add(duration).UnixNano())
	if !banNotified.Swap(true) && onBan != nil {
		onBan(duration)
	}
}

func (c *Client) Get(path string, params map[string]string) ([]byte, error) {
	return c.do("GET", path, params, nil, false)
}

func (c *Client) AuthGet(path string, params map[string]string) ([]byte, error) {
	return c.do("GET", path, params, nil, true)
}

func (c *Client) AuthPost(path string, body map[string]interface{}) ([]byte, error) {
	return c.do("POST", path, nil, body, true)
}

func (c *Client) do(method, path string, params map[string]string, body map[string]interface{}, sign bool) ([]byte, error) {
	const maxRetries = 5

	for attempt := 0; attempt < maxRetries; attempt++ {
		// 0. Глобальный IP-бан — ждём восстановления
		if banned, remaining := IsBanned(); banned {
			if remaining > 30*time.Second {
				return nil, fmt.Errorf("bybit: IP заблокирован, ждём %s", remaining.Round(time.Second))
			}
			time.Sleep(remaining)
		}

		// 1. Circuit breaker — проверяем не заблокированы ли мы
		if open, wait := c.breaker.isOpen(); open {
			time.Sleep(wait)
		}

		// 2. Rate limiter — ждём свободный токен
		c.limiter.wait()

		// 3. Задержка между запросами = фиксированная (BYBIT_REQUEST_DELAY_MS) + случайный джиттер ±20%
		// Фиксированная часть: из .env, default 500 мс
		// Джиттер: ±20% от задержки — ломает паттерн "ровно N мс между запросами"
		jitterRange := int(float64(requestDelay) * 0.20)
		if jitterRange < 50 {
			jitterRange = 50
		}
		delay := requestDelay + time.Duration(rand.Intn(jitterRange*2)-jitterRange)*time.Millisecond
		if delay < 100*time.Millisecond {
			delay = 100 * time.Millisecond
		}
		time.Sleep(delay)

		// 4. Выполняем запрос
		data, status, limitStatus, err := c.execute(method, path, params, body, sign)

		// 5. Адаптируем скорость на основе заголовка X-BAPI-LIMIT-STATUS
		//    Значение 0-100, где 100 = лимит исчерпан
		if limitStatus >= 80 {
			c.limiter.slowDown(0.5) // замедляемся на 50% на 30 сек
		} else if limitStatus >= 60 {
			c.limiter.slowDown(0.25)
		}

		if err != nil {
			// Сетевая ошибка — повтор с бэкоффом
			wait := backoff(attempt)
			time.Sleep(wait)
			continue
		}

		switch status {
		case 200:
			// Проверяем retCode в теле ответа
			if retErr := c.checkRetCode(data); retErr != nil {
				return nil, retErr
			}
			c.breaker.recordSuccess()
			return data, nil

		case 429:
			// Rate limit от HTTP-слоя
			c.breaker.recordFailure()
			wait := backoff(attempt)
			time.Sleep(wait)
			continue

		case 403:
			// IP-бан — долгая пауза, нет смысла повторять быстро
			return nil, fmt.Errorf("bybit 403: IP заблокирован, подождите ~10 минут")

		default:
			if status >= 500 {
				// Серверная ошибка Bybit — повтор
				wait := backoff(attempt)
				time.Sleep(wait)
				continue
			}
			return nil, fmt.Errorf("bybit %d: %s", status, string(data))
		}
	}

	return nil, fmt.Errorf("bybit: превышено число попыток (%d)", maxRetries)
}

// execute выполняет один HTTP-запрос и возвращает (тело, статус, limitPct, ошибка).
func (c *Client) execute(method, path string, params map[string]string, body map[string]interface{}, sign bool) ([]byte, int, int, error) {
	rawURL := c.baseURL + path
	var reqBody string
	var bodyReader io.Reader

	if method == "GET" && len(params) > 0 {
		rawURL += "?" + qs(params)
	}
	if method == "POST" && len(body) > 0 {
		reqBody = jsonStr(body)
		bodyReader = strings.NewReader(reqBody)
	}

	req, err := http.NewRequest(method, rawURL, bodyReader)
	if err != nil {
		return nil, 0, 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	if sign {
		ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
		recv := "5000"
		payload := ts + c.apiKey + recv
		if method == "GET" {
			payload += qs(params)
		} else {
			payload += reqBody
		}
		mac := hmac.New(sha256.New, []byte(c.apiSecret))
		mac.Write([]byte(payload))
		req.Header.Set("X-BAPI-API-KEY", c.apiKey)
		req.Header.Set("X-BAPI-TIMESTAMP", ts)
		req.Header.Set("X-BAPI-RECV-WINDOW", recv)
		req.Header.Set("X-BAPI-SIGN", hex.EncodeToString(mac.Sum(nil)))
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, 0, err
	}
	defer resp.Body.Close()

	// Читаем заголовок использования лимитов (0-100)
	limitPct := 0
	if h := resp.Header.Get("X-BAPI-LIMIT-STATUS"); h != "" {
		limitPct, _ = strconv.Atoi(h)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, limitPct, err
	}
	return data, resp.StatusCode, limitPct, nil
}

// checkRetCode проверяет retCode в JSON-теле ответа Bybit.
func (c *Client) checkRetCode(data []byte) error {
	var resp struct {
		RetCode int    `json:"retCode"`
		RetMsg  string `json:"retMsg"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil
	}
	switch resp.RetCode {
	case 0:
		return nil
	case 10006:
		// IP заблокирован — замораживаем все запросы на 10 минут
		triggerBan(10*time.Minute, c.onBan)
		return fmt.Errorf("bybit: IP заблокирован (10006) — пауза 10 минут. Установите PROXY_URL в .env")
	case 10018:
		// Rate limit превышен — замораживаем на 1 минуту
		triggerBan(60*time.Second, c.onBan)
		return fmt.Errorf("bybit: rate limit (10018) — пауза 60 сек. Увеличьте SCAN_INTERVAL_SEC")
	case 10016:
		triggerBan(30*time.Second, c.onBan)
		return fmt.Errorf("bybit: слишком много запросов (10016) — пауза 30 сек")
	default:
		if resp.RetCode != 0 {
			return fmt.Errorf("bybit retCode %d: %s", resp.RetCode, resp.RetMsg)
		}
		return nil
	}
}

// backoff возвращает время ожидания перед retry: 1s, 2s, 4s, 8s, 16s.
func backoff(attempt int) time.Duration {
	base := time.Duration(1<<uint(attempt)) * time.Second
	if base > 16*time.Second {
		base = 16 * time.Second
	}
	// Добавляем джиттер ±25% чтобы несколько инстансов не синхронизировались
	jitter := time.Duration(rand.Int63n(int64(base / 4)))
	return base + jitter
}

// ---- утилиты ----

func qs(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(params[k]))
	}
	return strings.Join(parts, "&")
}

func jsonStr(m map[string]interface{}) string {
	parts := make([]string, 0, len(m))
	for k, v := range m {
		parts = append(parts, fmt.Sprintf(`"%s":%s`, k, jsonVal(v)))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func jsonVal(v interface{}) string {
	switch val := v.(type) {
	case string:
		return `"` + val + `"`
	case bool:
		if val {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(val)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	default:
		return fmt.Sprintf(`"%v"`, val)
	}
}
